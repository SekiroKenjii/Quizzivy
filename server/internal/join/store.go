package join

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
)

var ErrClassNotFound = errors.New("join: class not found")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// IssuedCode is the metadata of an active code. It never carries the plaintext:
// that exists only in the response to the request that created it (§13.3).
type IssuedCode struct {
	ID        string
	ClassID   string
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

type RotateInput struct {
	ClassID     string
	ActorUserID string
	CodeHash    []byte
	Hint        string
	ExpiresAt   time.Time
	MaxUses     *int
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// Rotate revokes the class's active code and issues a replacement, in ONE
// transaction (§6.1).
//
// The transaction is not a nicety. `class_join_codes_one_active` is a partial
// unique index on (class_id) WHERE revoked_at IS NULL, so inserting before
// revoking violates it and revoking before inserting leaves a window in which
// the class has no code at all. Doing both in one transaction means neither
// state is ever observable.
//
// Previously enrolled students are untouched: membership lives in
// class_members and has no reference to the code that produced it beyond a
// historical join_code_id.
func (s *Store) Rotate(ctx context.Context, in RotateInput) (IssuedCode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return IssuedCode{}, fmt.Errorf("begin rotate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the class row, so two admins rotating at once serialise here rather
	// than racing into the partial unique index and one of them getting a
	// constraint violation instead of a code.
	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT true FROM app.classes WHERE id = $1 FOR UPDATE`,
		in.ClassID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return IssuedCode{}, ErrClassNotFound
	}
	if err != nil {
		return IssuedCode{}, fmt.Errorf("lock class: %w", err)
	}

	const revokeActive = `
		UPDATE app.class_join_codes
		   SET revoked_at = $2
		 WHERE class_id = $1 AND revoked_at IS NULL
		RETURNING id::text`
	var replaced *string
	var previousID string
	if err := tx.QueryRow(ctx, revokeActive, in.ClassID, in.Now).Scan(&previousID); err == nil {
		replaced = &previousID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return IssuedCode{}, fmt.Errorf("revoke previous code: %w", err)
	}

	const issue = `
		INSERT INTO app.class_join_codes
		       (class_id, code_hash, code_hint, expires_at, max_uses, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, uses_count`
	out := IssuedCode{
		ClassID:   in.ClassID,
		Hint:      in.Hint,
		ExpiresAt: in.ExpiresAt,
		MaxUses:   in.MaxUses,
	}
	if err := tx.QueryRow(ctx, issue,
		in.ClassID, in.CodeHash, in.Hint, in.ExpiresAt, in.MaxUses, in.ActorUserID, in.Now,
	).Scan(&out.ID, &out.UsesCount); err != nil {
		return IssuedCode{}, fmt.Errorf("issue code: %w", err)
	}

	// Issuing a code IS enabling self-join. Leaving the flag alone would mean a
	// rotate after a revoke hands the teacher a code that silently does
	// nothing -- §6.4 turns the flag off on revoke, so something has to turn it
	// back on, and this is the action that means "let students in again".
	if _, err := tx.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = true WHERE id = $1`, in.ClassID); err != nil {
		return IssuedCode{}, fmt.Errorf("enable self join: %w", err)
	}

	action := "class.join_code_issued"
	if replaced != nil {
		action = "class.join_code_rotated"
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      action,
		Entity:      "class_join_code",
		EntityID:    &out.ID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return IssuedCode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return IssuedCode{}, fmt.Errorf("commit rotate: %w", err)
	}
	return out, nil
}

type RevokeInput struct {
	ClassID     string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// Revoke ends the active code without issuing a replacement, and turns off
// self-join (§6.4).
//
// Both halves, or neither. Revoking the code while leaving self_join_enabled
// true would advertise a join flow that cannot succeed; clearing the flag
// without revoking would leave a live bearer secret in circulation that the
// teacher believes they have cancelled.
//
// Idempotent: revoking a class that has no active code still clears the flag
// and still returns success. "There is no way in" is the requested state, and
// reporting failure would invite a retry that changes nothing.
func (s *Store) Revoke(ctx context.Context, in RevokeInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revoke: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT true FROM app.classes WHERE id = $1 FOR UPDATE`,
		in.ClassID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrClassNotFound
	}
	if err != nil {
		return fmt.Errorf("lock class: %w", err)
	}

	var revokedID *string
	var id string
	err = tx.QueryRow(ctx,
		`UPDATE app.class_join_codes SET revoked_at = $2
		  WHERE class_id = $1 AND revoked_at IS NULL
		 RETURNING id::text`, in.ClassID, in.Now).Scan(&id)
	if err == nil {
		revokedID = &id
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("revoke code: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = false WHERE id = $1`, in.ClassID); err != nil {
		return fmt.Errorf("disable self join: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.join_code_revoked",
		Entity:      "class_join_code",
		// Nil when there was nothing to revoke: the class was already closed,
		// and inventing an id would make the log say otherwise.
		EntityID:   revokedID,
		OccurredAt: in.Now,
		IP:         in.IP,
		UserAgent:  in.UserAgent,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ActiveCode returns the class's live code metadata, or nil if there is none.
func (s *Store) ActiveCode(ctx context.Context, classID string) (*IssuedCode, error) {
	const q = `
		SELECT id::text, class_id::text, code_hint, expires_at, max_uses, uses_count
		  FROM app.class_join_codes
		 WHERE class_id = $1 AND revoked_at IS NULL`

	var c IssuedCode
	err := s.pool.QueryRow(ctx, q, classID).Scan(
		&c.ID, &c.ClassID, &c.Hint, &c.ExpiresAt, &c.MaxUses, &c.UsesCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load active join code: %w", err)
	}
	return &c, nil
}
