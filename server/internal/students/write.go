package students

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/audit"
)

// Request is the admin behind a write, for the audit row.
type Request struct {
	ActorID   string
	IP        string
	UserAgent string
}

type CreateInput struct {
	Email    string
	FullName string
	ClassIDs []string
	// Hash is computed by the caller: Argon2id blocks on a four-slot semaphore
	// that must not be held across a transaction.
	Hash string
	Now  time.Time
}

type UpdateInput struct {
	ID string
	// nil means the caller did not send the field.
	FullName *string
	Email    *string
	Disabled *bool
	Now      time.Time
}

// isUniqueViolation reports a collision with the case-insensitive email index.
//
// There is no UNIQUE constraint on the column -- uniqueness comes from
// users_email_lower_key, an expression index -- so `ON CONFLICT (email)` does
// not compile and the error has to be read after the fact.
func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" &&
		pg.ConstraintName == "users_email_lower_key"
}

// Create adds a student who signs in with a temporary password (§6.3: only
// Google self-signup exists, so an admin-created account has to carry one).
func (s *Store) Create(ctx context.Context, req Request, in CreateInput) (Student, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Student{}, fmt.Errorf("students: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role, password_hash, must_change_password)
		VALUES ($1, $2, 'student', $3, true)
		RETURNING id::text`, in.Email, in.FullName, in.Hash).Scan(&id)
	if isUniqueViolation(err) {
		return Student{}, ErrEmailTaken
	}
	if err != nil {
		return Student{}, fmt.Errorf("students: insert: %w", err)
	}

	for _, classID := range in.ClassIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)
			ON CONFLICT (class_id, user_id) DO NOTHING`,
			classID, id, req.ActorID); err != nil {
			return Student{}, fmt.Errorf("students: enrol: %w", err)
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "student.created",
		Entity:      "user",
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	}); err != nil {
		return Student{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Student{}, fmt.Errorf("students: commit create: %w", err)
	}
	return s.Get(ctx, id)
}

// Update edits profile fields, or disables the account.
//
// Disabling never deletes: app.users has no deleted_at, and attempts reference
// users with ON DELETE RESTRICT, so the history a disabled student leaves
// behind is exactly what must survive (§6.4).
func (s *Store) Update(ctx context.Context, req Request, in UpdateInput) (Student, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Student{}, fmt.Errorf("students: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE app.users
		   SET full_name   = coalesce($2, full_name),
		       email       = coalesce($3, email),
		       disabled_at = CASE
		                       WHEN $4::boolean IS NULL THEN disabled_at
		                       WHEN $4 THEN coalesce(disabled_at, $5)
		                       ELSE NULL
		                     END
		 WHERE id = $1::uuid AND role = 'student'`,
		in.ID, in.FullName, in.Email, in.Disabled, in.Now)
	if isUniqueViolation(err) {
		return Student{}, ErrEmailTaken
	}
	if err != nil {
		return Student{}, fmt.Errorf("students: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Student{}, ErrNotFound
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "student.updated",
		Entity:      "user",
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	}); err != nil {
		return Student{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Student{}, fmt.Errorf("students: commit update: %w", err)
	}
	// includeDisabled: a successful disable must return the row it just wrote, not 404.
	return s.get(ctx, in.ID, true)
}

// ResetPassword sets a temporary password and revokes every session the student
// has.
//
// All of them, with no exception carved: the acting principal is the admin, so
// unlike a self-service change there is no caller session worth preserving --
// and a reset performed because access may be compromised is worthless if the
// attacker's refresh family survives it.
func (s *Store) ResetPassword(ctx context.Context, req Request, id, hash string, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("students: begin reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE app.users
		   SET password_hash = $2, must_change_password = true
		 WHERE id = $1::uuid AND role = 'student' AND disabled_at IS NULL`, id, hash)
	if err != nil {
		return fmt.Errorf("students: reset password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE app.refresh_tokens
		   SET revoked_at = $2
		 WHERE user_id = $1::uuid AND revoked_at IS NULL`, id, now); err != nil {
		return fmt.Errorf("students: revoke sessions: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "student.password_reset",
		Entity:      "user",
		EntityID:    &id,
		OccurredAt:  now,
		IP:          optional(req.IP),
		UserAgent:   optional(req.UserAgent),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
