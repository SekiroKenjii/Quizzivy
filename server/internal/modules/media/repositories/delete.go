package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/audit"

	"github.com/jackc/pgx/v5"
)

// SoftDelete marks an unreferenced asset deleted and audits it.
//
// The object in R2 is deliberately left in place: §15 scopes lifecycle cleanup
// out of v1, and an asset row can still be referenced by a frozen test version
// whose file must keep resolving. Soft delete removes it from the library, not
// from storage.
func (s *Postgres) SoftDelete(ctx context.Context, in domain.DeleteInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("media: begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var alreadyDeleted bool
	err = tx.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.media_assets WHERE id = $1 FOR UPDATE`,
		in.ID).Scan(&alreadyDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("media: lock asset: %w", err)
	}
	if alreadyDeleted {
		return domain.ErrNotFound
	}
	refs, err := References(ctx, tx, in.ID)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return &domain.ReferencedError{Tests: refs}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE app.media_assets SET deleted_at = $2 WHERE id = $1`,
		in.ID, in.Now); err != nil {
		return fmt.Errorf("media: soft delete: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "media.deleted",
		Entity:      "media_asset",
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("media: commit delete: %w", err)
	}
	return nil
}
