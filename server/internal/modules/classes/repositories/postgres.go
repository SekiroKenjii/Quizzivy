package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/paging"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const entityClassMember = "class_member"

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

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

func (s *Postgres) Get(ctx context.Context, classID string) (domain.Class, error) {
	c, err := scanClass(s.pool.QueryRow(ctx, classProjection+` WHERE c.id = $1`, classID))
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.Class{}, fmt.Errorf("load class %s: %w", classID, err)
	}
	return c, err
}

// List returns one page of classes, newest first, with the paging beside it
// (O-20). §1.3 promised single-digit classes; a development database already
// holds over a hundred, which is what broke the pickers reading this whole.
func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Class, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args, where := searchClause(in.Query)
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
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM app.classes c
	  LEFT JOIN app.class_join_codes jc ON jc.class_id = c.id AND jc.revoked_at IS NULL`+condition,
		args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("count classes: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, classProjection+condition+fmt.Sprintf(
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

func searchClause(query string) ([]any, []string) {
	var args []any
	where := []string{"TRUE"}
	if q := strings.TrimSpace(query); q != "" {
		args = append(args, likeEscaper.Replace(q))
		where = append(where, fmt.Sprintf(nameSearch, len(args)))
	}
	return args, where
}

func (s *Postgres) Facets(ctx context.Context, query string) (domain.Facets, error) {
	args, where := searchClause(query)
	var f domain.Facets
	err := s.pool.QueryRow(ctx, `
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
// whose hint is the teacher's, and never the roster. The teacher is the
// practice's one admin account (§1.1).
func (s *Postgres) ListMine(ctx context.Context, userID string) ([]domain.MyClass, error) {
	rows, err := s.pool.Query(ctx, `
	SELECT c.id::text, c.name, c.description, me.joined_at,
	       (SELECT t.full_name FROM app.users t
	         WHERE t.role = 'admin' AND t.disabled_at IS NULL
	         ORDER BY t.created_at, t.id LIMIT 1)
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

// Members lists who is in the class and HOW they got in.
func (s *Postgres) Members(ctx context.Context, classID string, in domain.MembersInput) ([]domain.Member, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args := []any{classID}
	where := []string{`m.class_id = $1`}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, likeEscaper.Replace(q))
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
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("count members of %s: %w", classID, err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, memberColumns+`
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

// RemoveMember revokes access. It does NOT touch attempts (§6.4).
func (s *Postgres) RemoveMember(ctx context.Context, in domain.RemoveMemberInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin remove member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`DELETE FROM app.class_members WHERE class_id = $1 AND user_id = $2`,
		in.ClassID, in.UserID)
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

// Update edits a class's own fields.
func (s *Postgres) Update(ctx context.Context, classID string, in domain.UpdateInput) (domain.Class, error) {
	sets := []string{}
	args := []any{classID}

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
		return s.Get(ctx, classID)
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE app.classes SET `+strings.Join(sets, ", ")+` WHERE id = $1`, args...)
	if err != nil {
		return domain.Class{}, fmt.Errorf("update class %s: %w", classID, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Class{}, domain.ErrNotFound
	}
	return s.Get(ctx, classID)
}

func (s *Postgres) Create(ctx context.Context, in domain.CreateInput) (domain.Class, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Class{}, fmt.Errorf("begin create class: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO app.classes (name, description, self_join_enabled, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text`,
		in.Name, in.Description, in.SelfJoinEnabled, in.Now).Scan(&id); err != nil {
		return domain.Class{}, fmt.Errorf("create class: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.created",
		Entity:      "class",
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
	return s.Get(ctx, id)
}

// Archive sets or clears archived_at. Idempotent: a repeat is not audited.
func (s *Postgres) Archive(ctx context.Context, in domain.ArchiveInput) (domain.Class, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Class{}, fmt.Errorf("begin archive class: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var changed bool
	err = tx.QueryRow(ctx, `
		UPDATE app.classes
		   SET archived_at = CASE WHEN $2 THEN coalesce(archived_at, $3::timestamptz) END
		 WHERE id = $1::uuid
		RETURNING (old.archived_at IS NULL) <> (new.archived_at IS NULL)`,
		in.ClassID, in.Archived, in.Now).Scan(&changed)
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
			Entity:      "class",
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
	return s.Get(ctx, in.ClassID)
}

// AddMember enrols an existing student directly, as joined_via 'admin'.
func (s *Postgres) AddMember(ctx context.Context, in domain.AddMemberInput) (domain.Member, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Member{}, fmt.Errorf("begin add member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.classes WHERE id = $1::uuid)`,
		in.ClassID).Scan(&exists); err != nil {
		return domain.Member{}, fmt.Errorf("add member: %w", err)
	}
	if !exists {
		return domain.Member{}, domain.ErrNotFound
	}

	var role string
	switch err := tx.QueryRow(ctx,
		`SELECT role::text FROM app.users WHERE id = $1::uuid AND disabled_at IS NULL`,
		in.UserID).Scan(&role); {
	case err == nil && role == "student":
	case err == nil, errors.Is(err, pgx.ErrNoRows):
		return domain.Member{}, domain.ErrNotAStudent
	default:
		return domain.Member{}, fmt.Errorf("add member: %w", err)
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
