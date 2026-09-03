package classes

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/paging"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
)

var ErrNotFound = errors.New("classes: not found")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListInput selects a page of classes. Query matches the name, accent-folded
// on both sides like every other search here (D-11).
type ListInput struct {
	Query string
	Page  int
	Limit int
}

// MembersInput selects a page of one class's roster. Query matches name or
// email.
type MembersInput struct {
	Query string
	Page  int
	Limit int
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

const nameSearch = `app.immutable_unaccent(lower(c.name))` +
	` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'`

// JoinCodeInfo is metadata about the ACTIVE code -- never the code itself.
// The plaintext exists once, in the response that created it (§13.3).
type JoinCodeInfo struct {
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

type Class struct {
	ID              string
	Name            string
	Description     *string
	StudentCount    int
	SelfJoinEnabled bool
	CreatedAt       time.Time
	JoinCode        *JoinCodeInfo
}

type Member struct {
	UserID       string
	FullName     string
	Email        string
	JoinedVia    string
	JoinedAt     time.Time
	JoinCodeHint *string
}

const classProjection = `
	SELECT c.id::text, c.name, c.description, c.self_join_enabled, c.created_at,
	       -- Live members only. A disabled account cannot sign in, so counting
	       -- it makes every assignment on this class read one short for ever.
	       (SELECT count(*) FROM app.class_members m
	          JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
	         WHERE m.class_id = c.id),
	       jc.code_hint, jc.expires_at, jc.max_uses, jc.uses_count
	  FROM app.classes c
	  -- The active code, if there is one. LEFT JOIN because a class with
	  -- self-join closed has none, and that is a normal state rather than a
	  -- missing row.
	  LEFT JOIN app.class_join_codes jc
	         ON jc.class_id = c.id AND jc.revoked_at IS NULL`

func scanClass(row pgx.Row) (Class, error) {
	var c Class
	var hint *string
	var expires *time.Time
	var maxUses *int
	var uses *int

	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.SelfJoinEnabled, &c.CreatedAt,
		&c.StudentCount, &hint, &expires, &maxUses, &uses)
	if errors.Is(err, pgx.ErrNoRows) {
		return Class{}, ErrNotFound
	}
	if err != nil {
		return Class{}, err
	}
	if hint != nil && expires != nil && uses != nil {
		c.JoinCode = &JoinCodeInfo{
			Hint: *hint, ExpiresAt: *expires, MaxUses: maxUses, UsesCount: *uses,
		}
	}
	return c, nil
}

func (s *Store) Get(ctx context.Context, classID string) (Class, error) {
	c, err := scanClass(s.pool.QueryRow(ctx, classProjection+` WHERE c.id = $1`, classID))
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Class{}, fmt.Errorf("load class %s: %w", classID, err)
	}
	return c, err
}

// List returns one page of classes, newest first, with the paging beside it
// (O-20). §1.3 promised single-digit classes; a development database already
// holds over a hundred, which is what broke the pickers reading this whole.
func (s *Store) List(ctx context.Context, in ListInput) ([]Class, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	var args []any
	where := []string{"TRUE"}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, likeEscaper.Replace(q))
		where = append(where, fmt.Sprintf(nameSearch, len(args)))
	}
	condition := " WHERE " + strings.Join(where, " AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM app.classes c`+condition, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("count classes: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, classProjection+condition+fmt.Sprintf(
		` ORDER BY c.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("list classes: %w", err)
	}
	defer rows.Close()

	out := make([]Class, 0, limit)
	for rows.Next() {
		c, err := scanClass(rows)
		if err != nil {
			return nil, paging.Page{}, fmt.Errorf("list classes: %w", err)
		}
		out = append(out, c)
	}
	return out, page, rows.Err()
}

// ListMine is §9's /app/classes: the classes this student belongs to, most
// recently joined first. The join code is never populated -- its hint is the
// teacher's, and a student who can read four characters of it has four
// characters more than they should.
func (s *Store) ListMine(ctx context.Context, userID string) ([]Class, error) {
	rows, err := s.pool.Query(ctx, classProjection+`
	  JOIN app.class_members me ON me.class_id = c.id AND me.user_id = $1::uuid
	  JOIN app.users student ON student.id = me.user_id AND student.disabled_at IS NULL
	 ORDER BY me.joined_at DESC, c.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list my classes: %w", err)
	}
	defer rows.Close()

	var out []Class
	for rows.Next() {
		c, err := scanClass(rows)
		if err != nil {
			return nil, fmt.Errorf("list my classes: %w", err)
		}
		c.JoinCode = nil
		out = append(out, c)
	}
	return out, rows.Err()
}

// Members lists who is in the class and HOW they got in.
//
// joined_via and the code hint are the point (§6.4): they are what lets a
// teacher spot an unexpected enrolment, which is the mitigation §17.2 chose
// instead of building an approval queue.
func (s *Store) Members(ctx context.Context, classID string, in MembersInput) ([]Member, paging.Page, error) {
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
	rows, err := s.pool.Query(ctx, `
		SELECT u.id::text, u.full_name, u.email, m.joined_via::text, m.joined_at, jc.code_hint`+from+
		fmt.Sprintf(`
		 -- u.id breaks joined_at ties, or a row lands on two pages.
		 ORDER BY m.joined_at DESC, u.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("list members of %s: %w", classID, err)
	}
	defer rows.Close()

	out := make([]Member, 0, limit)
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Email, &m.JoinedVia, &m.JoinedAt, &m.JoinCodeHint); err != nil {
			return nil, paging.Page{}, fmt.Errorf("list members of %s: %w", classID, err)
		}
		out = append(out, m)
	}
	return out, page, rows.Err()
}

type RemoveMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// RemoveMember revokes access. It does NOT touch attempts (§6.4).
func (s *Store) RemoveMember(ctx context.Context, in RemoveMemberInput) error {
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
		Entity:      "class_member",
		EntityID:    &in.ClassID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateInput carries only the fields the caller actually sent, so a PATCH that
// renames a class cannot silently clear its description.
type UpdateInput struct {
	Name *string
	// nil means "the caller did not send it".
	Description     *string
	SelfJoinEnabled *bool
}

// Update edits a class's own fields.
//
// Disabling self-join deliberately does NOT revoke the code: §6.4 separates the
// two, and a teacher pausing enrolment for a week should not have to reissue a
// code and re-share it afterwards. Revoking is its own endpoint.
func (s *Store) Update(ctx context.Context, classID string, in UpdateInput) (Class, error) {
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
		return Class{}, fmt.Errorf("update class %s: %w", classID, err)
	}
	if tag.RowsAffected() == 0 {
		return Class{}, ErrNotFound
	}
	return s.Get(ctx, classID)
}

type AddMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

var ErrNotAStudent = errors.New("classes: not a student")

// AddMember enrols an existing student directly, as joined_via 'admin'.
//
// Idempotent like RemoveMember: adding someone already in the class returns
// their existing row rather than an error, because the teacher asked for a
// state and that state already holds. The second add is not audited -- nothing
// changed, and an audit log that records non-events is one nobody reads.
func (s *Store) AddMember(ctx context.Context, in AddMemberInput) (Member, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Member{}, fmt.Errorf("begin add member: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM app.classes WHERE id = $1::uuid)`,
		in.ClassID).Scan(&exists); err != nil {
		return Member{}, fmt.Errorf("add member: %w", err)
	}
	if !exists {
		return Member{}, ErrNotFound
	}

	var role string
	switch err := tx.QueryRow(ctx,
		`SELECT role::text FROM app.users WHERE id = $1::uuid AND disabled_at IS NULL`,
		in.UserID).Scan(&role); {
	case err == nil && role == "student":
	case err == nil, errors.Is(err, pgx.ErrNoRows):
		return Member{}, ErrNotAStudent
	default:
		return Member{}, fmt.Errorf("add member: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)
		ON CONFLICT (class_id, user_id) DO NOTHING`,
		in.ClassID, in.UserID, in.ActorUserID)
	if err != nil {
		return Member{}, fmt.Errorf("add member: %w", err)
	}

	if tag.RowsAffected() > 0 {
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &in.ActorUserID,
			Action:      "class.member_added",
			Entity:      "class_member",
			EntityID:    &in.ClassID,
			OccurredAt:  in.Now,
			IP:          in.IP,
			UserAgent:   in.UserAgent,
		}); err != nil {
			return Member{}, err
		}
	}

	var m Member
	if err := tx.QueryRow(ctx, `
		SELECT u.id::text, u.full_name, u.email, m.joined_via::text, m.joined_at, jc.code_hint
		  FROM app.class_members m
		  JOIN app.users u ON u.id = m.user_id
		  LEFT JOIN app.class_join_codes jc ON jc.id = m.join_code_id
		 WHERE m.class_id = $1::uuid AND m.user_id = $2::uuid`,
		in.ClassID, in.UserID).
		Scan(&m.UserID, &m.FullName, &m.Email, &m.JoinedVia, &m.JoinedAt, &m.JoinCodeHint); err != nil {
		return Member{}, fmt.Errorf("add member: read back: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Member{}, fmt.Errorf("commit add member: %w", err)
	}
	return m, nil
}
