package classes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
)

var ErrNotFound = errors.New("classes: not found")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

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
	       (SELECT count(*) FROM app.class_members m WHERE m.class_id = c.id),
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

func (s *Store) List(ctx context.Context) ([]Class, error) {
	rows, err := s.pool.Query(ctx, classProjection+` ORDER BY c.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list classes: %w", err)
	}
	defer rows.Close()

	var out []Class
	for rows.Next() {
		c, err := scanClass(rows)
		if err != nil {
			return nil, fmt.Errorf("list classes: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Members lists who is in the class and HOW they got in.
//
// joined_via and the code hint are the point (§6.4): they are what lets a
// teacher spot an unexpected enrolment, which is the mitigation §17.2 chose
// instead of building an approval queue.
func (s *Store) Members(ctx context.Context, classID string) ([]Member, error) {
	const q = `
		SELECT u.id::text, u.full_name, u.email, m.joined_via::text, m.joined_at, jc.code_hint
		  FROM app.class_members m
		  JOIN app.users u ON u.id = m.user_id
		  LEFT JOIN app.class_join_codes jc ON jc.id = m.join_code_id
		 WHERE m.class_id = $1
		 ORDER BY m.joined_at DESC`

	rows, err := s.pool.Query(ctx, q, classID)
	if err != nil {
		return nil, fmt.Errorf("list members of %s: %w", classID, err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Email, &m.JoinedVia, &m.JoinedAt, &m.JoinCodeHint); err != nil {
			return nil, fmt.Errorf("list members of %s: %w", classID, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
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
//
// The membership row is what grants access; the attempts are the student's
// work and the teacher's record of it. Deleting them because someone left a
// class would destroy the only evidence of what happened -- and §6.4 says
// retain, so the FK from attempts does not point here at all.
//
// Idempotent: removing someone who is not a member leaves the class in the
// requested state and reports success.
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
	// nil means "the caller did not send it". Clearing a description back to
	// NULL is therefore not expressible: the generated body decodes both an
	// omitted field and an explicit null to a nil *string, so the handler
	// cannot tell them apart. Sending "" is how a teacher empties it.
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
