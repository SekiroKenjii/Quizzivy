package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/paging"
	"quizzivy/internal/shared/visibility"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const entityClassMember = "class_member"

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var anyClass = access.Scope{All: true}

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

const taughtClass = `($2::boolean OR teacher_id = $3::uuid)`

const nameSearch = `app.immutable_unaccent(lower(c.name))` +
	` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'`

const openAssignments = `
	       (SELECT count(*) FROM app.assignment_classes ac
	          JOIN app.assignments a ON a.id = ac.assignment_id
	         WHERE ac.class_id = c.id
	           AND a.published_at IS NOT NULL
	           AND NOT (a.closed_at IS NOT NULL AND now() >= a.closed_at)
	           AND now() >= a.opens_at AND now() < a.closes_at)`

const joinable = `(c.archived_at IS NULL AND c.self_join_enabled AND jc.expires_at > now()
	           AND (jc.max_uses IS NULL OR jc.uses_count < jc.max_uses))`

const classProjection = `
	SELECT c.id::text, c.name, c.description, c.self_join_enabled, c.archived_at, c.created_at,
	       -- Live members only. A disabled account cannot sign in, so counting
	       -- it makes every assignment on this class read one short for ever.
	       (SELECT count(*) FROM app.class_members m
	          JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
	         WHERE m.class_id = c.id),` + openAssignments + `,
	       jc.code_hint, jc.expires_at, jc.max_uses, jc.uses_count
	  FROM app.classes c
	  -- The active code, if there is one. LEFT JOIN because a class with
	  -- self-join closed has none, and that is a normal state rather than a
	  -- missing row.
	  LEFT JOIN app.class_join_codes jc
	         ON jc.class_id = c.id AND jc.revoked_at IS NULL`

func scanClass(row pgx.Row) (domain.Class, error) {
	var c domain.Class
	var hint *string
	var expires *time.Time
	var maxUses *int
	var uses *int

	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.SelfJoinEnabled, &c.ArchivedAt, &c.CreatedAt,
		&c.StudentCount, &c.OpenAssignmentCount, &hint, &expires, &maxUses, &uses)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Class{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Class{}, err
	}
	if hint != nil && expires != nil && uses != nil {
		c.JoinCode = &domain.JoinCodeInfo{
			Hint: *hint, ExpiresAt: *expires, MaxUses: maxUses, UsesCount: *uses,
		}
	}
	return c, nil
}

// Get returns one class the scope reaches; another teacher's answers
// ErrNotFound, exactly as a missing one does.
func (s *Postgres) Get(ctx context.Context, scope access.Scope, classID string) (domain.Class, error) {
	c, err := scanClass(s.QueryRow(ctx, classProjection+` WHERE c.id = $1 AND ($2::boolean OR c.teacher_id = $3::uuid)`,
		classID, scope.All, opt.String(scope.UserID)))
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.Class{}, fmt.Errorf("load class %s: %w", classID, err)
	}
	return c, err
}

// List returns one page of the classes the input's scope reaches, newest
// first, with the paging beside it (O-20). §1.3 promised single-digit classes;
// a development database already holds over a hundred, which is what broke the
// pickers reading this whole.
func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Class, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args, where := searchClause(in.Scope, in.Query)
	switch in.Status {
	case "archived":
		where = append(where, "c.archived_at IS NOT NULL")
	case "joinable":
		where = append(where, joinable)
	case "all":
	default:
		where = append(where, "c.archived_at IS NULL")
	}
	condition := " WHERE " + strings.Join(where, " AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*) FROM app.classes c
	  LEFT JOIN app.class_join_codes jc ON jc.class_id = c.id AND jc.revoked_at IS NULL`+condition,
		args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("count classes: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, classProjection+condition+fmt.Sprintf(
		` ORDER BY c.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("list classes: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Class, 0, limit)
	for rows.Next() {
		c, err := scanClass(rows)
		if err != nil {
			return nil, paging.Page{}, fmt.Errorf("list classes: %w", err)
		}
		out = append(out, c)
	}
	return out, page, rows.Err()
}

func searchClause(scope access.Scope, query string) ([]any, []string) {
	args, where := []any{}, []string{"TRUE"}
	if !scope.All {
		args = append(args, opt.String(scope.UserID))
		where = append(where, "c.teacher_id = $1::uuid")
	}
	if q := strings.TrimSpace(query); q != "" {
		args = append(args, db.EscapeLike(q))
		where = append(where, fmt.Sprintf(nameSearch, len(args)))
	}
	return args, where
}

// Facets counts the classes the scope reaches that match the search, and
// their live members.
func (s *Postgres) Facets(ctx context.Context, scope access.Scope, query string) (domain.Facets, error) {
	args, where := searchClause(scope, query)
	var f domain.Facets
	err := s.QueryRow(ctx, `
	SELECT count(*),
	       count(*) FILTER (WHERE `+joinable+`),
	       count(*) FILTER (WHERE c.archived_at IS NOT NULL),
	       (SELECT count(DISTINCT m.user_id) FROM app.class_members m
	          JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
	         WHERE m.class_id IN (SELECT c.id FROM app.classes c WHERE `+strings.Join(where, " AND ")+`))
	  FROM app.classes c
	  LEFT JOIN app.class_join_codes jc ON jc.class_id = c.id AND jc.revoked_at IS NULL
	 WHERE `+strings.Join(where, " AND "), args...).Scan(&f.All, &f.Joinable, &f.Archived, &f.Students)
	if err != nil {
		return domain.Facets{}, fmt.Errorf("count class facets: %w", err)
	}
	return f, nil
}

// ListMine is §9's /app/classes: the classes this student belongs to, most
// recently joined first, in the student's own shape -- never the join code,
// whose hint is the teacher's, and never the roster. The teacher named is each
// class's own.
func (s *Postgres) ListMine(ctx context.Context, userID string) ([]domain.MyClass, error) {
	rows, err := s.Query(ctx, `
	SELECT c.id::text, c.name, c.description, me.joined_at,
	       (SELECT coalesce(t.display_name, t.full_name) FROM app.users t WHERE t.id = c.teacher_id)
	  FROM app.classes c
	  JOIN app.class_members me ON me.class_id = c.id AND me.user_id = $1::uuid
	  JOIN app.users student ON student.id = me.user_id AND student.disabled_at IS NULL
	 WHERE c.archived_at IS NULL
	 ORDER BY me.joined_at DESC, c.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list my classes: %w", err)
	}
	defer rows.Close()

	var out []domain.MyClass
	for rows.Next() {
		var c domain.MyClass
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.JoinedAt, &c.TeacherName); err != nil {
			return nil, fmt.Errorf("list my classes: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Members lists who is in a class the scope reaches and HOW they got in.
// Another teacher's class lists nobody, exactly as a missing one does.
func (s *Postgres) Members(ctx context.Context, scope access.Scope, classID string, in domain.MembersInput) ([]domain.Member, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args := []any{classID, scope.All, opt.String(scope.UserID)}
	where := []string{`m.class_id = $1`, `EXISTS (SELECT 1 FROM app.classes c WHERE c.id = $1::uuid AND ($2::boolean OR c.teacher_id = $3::uuid))`}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, db.EscapeLike(q))
		where = append(where, fmt.Sprintf(`(app.immutable_unaccent(lower(u.full_name))
		           LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
		        OR lower(u.email) LIKE '%%' || lower($%[1]d) || '%%' ESCAPE '\')`, len(args)))
	}
	from := `
		  FROM app.class_members m
		  JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
		  LEFT JOIN app.class_join_codes jc ON jc.id = m.join_code_id
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("count members of %s: %w", classID, err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, memberColumns+`
		  FROM app.class_members m
		  JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
		  LEFT JOIN app.class_join_codes jc ON jc.id = m.join_code_id
		 WHERE `+strings.Join(where, "\n		   AND ")+
		fmt.Sprintf(`
		 -- u.id breaks joined_at ties, or a row lands on two pages.
		 ORDER BY m.joined_at DESC, u.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("list members of %s: %w", classID, err)
	}
	defer rows.Close()

	out := make([]domain.Member, 0, limit)
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, paging.Page{}, fmt.Errorf("list members of %s: %w", classID, err)
		}
		out = append(out, m)
	}
	return out, page, rows.Err()
}

const memberColumns = `
		SELECT u.id::text, u.full_name, u.email, m.joined_via::text, m.joined_at, jc.code_hint`

func scanMember(row pgx.Row) (domain.Member, error) {
	var m domain.Member
	if err := row.Scan(&m.UserID, &m.FullName, &m.Email, &m.JoinedVia, &m.JoinedAt, &m.JoinCodeHint); err != nil {
		return domain.Member{}, err
	}
	return m, nil
}

// RemoveMember revokes access to a class the actor teaches. It does NOT touch
// attempts (§6.4).
func (s *Postgres) RemoveMember(ctx context.Context, in domain.RemoveMemberInput) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`DELETE FROM app.class_members m USING app.classes c
		  WHERE m.class_id = $1 AND m.user_id = $4 AND c.id = m.class_id
		    AND ($2::boolean OR c.teacher_id = $3::uuid)`,
		in.ClassID, in.All, opt.String(in.ActorUserID), in.UserID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.member_removed",
		Entity:      entityClassMember,
		EntityID:    &in.ClassID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Update edits the own fields of a class the scope reaches; another teacher's
// class answers ErrNotFound, even when nothing would change.
func (s *Postgres) Update(ctx context.Context, scope access.Scope, classID string, in domain.UpdateInput) (domain.Class, error) {
	sets := []string{}
	args := []any{classID, scope.All, opt.String(scope.UserID)}

	if in.Name != nil {
		args = append(args, *in.Name)
		sets = append(sets, fmt.Sprintf("name = $%d", len(args)))
	}
	if in.Description != nil {
		args = append(args, *in.Description)
		sets = append(sets, fmt.Sprintf("description = $%d", len(args)))
	}
	if in.SelfJoinEnabled != nil {
		args = append(args, *in.SelfJoinEnabled)
		sets = append(sets, fmt.Sprintf("self_join_enabled = $%d", len(args)))
	}
	if len(sets) == 0 {
		return s.Get(ctx, scope, classID)
	}

	tag, err := s.Exec(ctx,
		`UPDATE app.classes SET `+strings.Join(sets, ", ")+` WHERE id = $1 AND `+taughtClass, args...)
	if err != nil {
		return domain.Class{}, fmt.Errorf("update class %s: %w", classID, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Class{}, domain.ErrNotFound
	}
	return s.Get(ctx, scope, classID)
}

func (s *Postgres) Create(ctx context.Context, in domain.CreateInput) (domain.Class, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Class{}, fmt.Errorf("begin create class: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.classes (name, description, self_join_enabled, created_at, teacher_id)
		VALUES ($1, $2, $3, $4, $5::uuid)
		RETURNING id::text`,
		in.Name, in.Description, in.SelfJoinEnabled, in.Now, in.ActorUserID).Scan(&id); err != nil {
		return domain.Class{}, fmt.Errorf("create class: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.created",
		Entity:      entityClass,
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return domain.Class{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Class{}, fmt.Errorf("commit create class: %w", err)
	}
	return s.Get(ctx, anyClass, id)
}

// Archive sets or clears archived_at on a class the actor teaches. Idempotent:
// a repeat is not audited.
func (s *Postgres) Archive(ctx context.Context, in domain.ArchiveInput) (domain.Class, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Class{}, fmt.Errorf("begin archive class: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var changed bool
	err = tx.QueryRow(ctx, `
		UPDATE app.classes
		   SET archived_at = CASE WHEN $4 THEN coalesce(archived_at, $5::timestamptz) END
		 WHERE id = $1::uuid AND `+taughtClass+`
		RETURNING (old.archived_at IS NULL) <> (new.archived_at IS NULL)`,
		in.ClassID, in.All, opt.String(in.ActorUserID), in.Archived, in.Now).Scan(&changed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Class{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Class{}, fmt.Errorf("archive class %s: %w", in.ClassID, err)
	}
	if changed {
		action := "class.restored"
		if in.Archived {
			action = "class.archived"
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &in.ActorUserID,
			Action:      action,
			Entity:      entityClass,
			EntityID:    &in.ClassID,
			OccurredAt:  in.Now,
			IP:          in.IP,
			UserAgent:   in.UserAgent,
		}); err != nil {
			return domain.Class{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Class{}, fmt.Errorf("commit archive class: %w", err)
	}
	return s.Get(ctx, anyClass, in.ClassID)
}

// AddMember enrols, as joined_via 'admin', an active student the actor
// reaches into a class the actor teaches. Another teacher's class answers
// ErrNotFound; a user who is not an active student the actor reaches answers
// ErrNotAStudent, whatever the reason.
func (s *Postgres) AddMember(ctx context.Context, in domain.AddMemberInput) (domain.Member, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Member{}, fmt.Errorf("begin add member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var locked bool
	err = tx.QueryRow(ctx, `SELECT true FROM app.classes WHERE id = $1::uuid AND `+taughtClass+` FOR SHARE`,
		in.ClassID, in.All, opt.String(in.ActorUserID)).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, fmt.Errorf("add member: %w", err)
	}

	var student bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.users u
		 WHERE u.id = $1::uuid AND u.disabled_at IS NULL
		   AND u.role_id IN (SELECT r.id FROM app.student_like_roles r)
		   AND ($2::boolean OR u.id IN `+visibility.StudentIDs(3)+`))`,
		in.UserID, in.All, opt.String(in.ActorUserID)).Scan(&student); err != nil {
		return domain.Member{}, fmt.Errorf("add member: %w", err)
	}
	if !student {
		return domain.Member{}, domain.ErrNotAStudent
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)
		ON CONFLICT (class_id, user_id) DO NOTHING`,
		in.ClassID, in.UserID, in.ActorUserID)
	if err != nil {
		return domain.Member{}, fmt.Errorf("add member: %w", err)
	}

	if tag.RowsAffected() > 0 {
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &in.ActorUserID,
			Action:      "class.member_added",
			Entity:      entityClassMember,
			EntityID:    &in.ClassID,
			OccurredAt:  in.Now,
			IP:          in.IP,
			UserAgent:   in.UserAgent,
		}); err != nil {
			return domain.Member{}, err
		}
	}

	m, err := scanMember(tx.QueryRow(ctx, memberColumns+`
		  FROM app.class_members m
		  JOIN app.users u ON u.id = m.user_id
		  LEFT JOIN app.class_join_codes jc ON jc.id = m.join_code_id
		 WHERE m.class_id = $1::uuid AND m.user_id = $2::uuid`,
		in.ClassID, in.UserID))
	if err != nil {
		return domain.Member{}, fmt.Errorf("add member: read back: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Member{}, fmt.Errorf("commit add member: %w", err)
	}
	return m, nil
}
