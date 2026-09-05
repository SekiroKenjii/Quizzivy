package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/audit"

	"github.com/jackc/pgx/v5"
)

// Rotate revokes the class's active code and issues a replacement in one
// transaction, so a class is never left with two active codes or none.
func (s *Postgres) Rotate(ctx context.Context, in domain.RotateInput) (domain.IssuedCode, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.IssuedCode{}, fmt.Errorf("begin rotate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT true FROM app.classes WHERE id = $1 FOR UPDATE`,
		in.ClassID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IssuedCode{}, domain.ErrClassNotFound
	}
	if err != nil {
		return domain.IssuedCode{}, fmt.Errorf("lock class: %w", err)
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
		return domain.IssuedCode{}, fmt.Errorf("revoke previous code: %w", err)
	}

	const issue = `
		INSERT INTO app.class_join_codes
		       (class_id, code_hash, code_hint, expires_at, max_uses, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, uses_count`
	out := domain.IssuedCode{
		ClassID:   in.ClassID,
		Hint:      in.Hint,
		ExpiresAt: in.ExpiresAt,
		MaxUses:   in.MaxUses,
	}
	if err := tx.QueryRow(ctx, issue,
		in.ClassID, in.CodeHash, in.Hint, in.ExpiresAt, in.MaxUses, in.ActorUserID, in.Now,
	).Scan(&out.ID, &out.UsesCount); err != nil {
		return domain.IssuedCode{}, fmt.Errorf("issue code: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = true WHERE id = $1`, in.ClassID); err != nil {
		return domain.IssuedCode{}, fmt.Errorf("enable self join: %w", err)
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
		return domain.IssuedCode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.IssuedCode{}, fmt.Errorf("commit rotate: %w", err)
	}
	return out, nil
}

// Revoke ends the active code without issuing a replacement, and turns off
// self-join (§6.4).
func (s *Postgres) Revoke(ctx context.Context, in domain.RevokeInput) error {
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
		return domain.ErrClassNotFound
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
		EntityID:    revokedID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ActiveCode returns the class's live code metadata, or nil if there is none.
func (s *Postgres) ActiveCode(ctx context.Context, classID string) (*domain.IssuedCode, error) {
	const q = `
		SELECT id::text, class_id::text, code_hint, expires_at, max_uses, uses_count
		  FROM app.class_join_codes
		 WHERE class_id = $1 AND revoked_at IS NULL`

	var c domain.IssuedCode
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
