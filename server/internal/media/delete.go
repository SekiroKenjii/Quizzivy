package media

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/audit"
)

// ErrReferenced is a delete refused because a published version still uses the
// asset (§8, §15). Answered 409, not 403: the caller has every right to the
// asset, the asset is simply not deletable while something depends on it.
var ErrReferenced = errors.New("media: asset is referenced by a published version")

// DeleteInput is one soft delete, with the audit context it must record.
type DeleteInput struct {
	ID        string
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}

// SoftDelete marks an unreferenced asset deleted and audits it.
//
// The object in R2 is deliberately left in place: §15 scopes lifecycle cleanup
// out of v1, and an asset row can still be referenced by a frozen test version
// whose file must keep resolving. Soft delete removes it from the library, not
// from storage.
func (s *Store) SoftDelete(ctx context.Context, in DeleteInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("media: begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Locked before the reference count, not after. Without the lock, a publish
	// committing between the count and the update would leave a live version
	// pointing at a deleted asset -- the exact state the 409 exists to prevent.
	var alreadyDeleted bool
	err = tx.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.media_assets WHERE id = $1 FOR UPDATE`,
		in.ID).Scan(&alreadyDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("media: lock asset: %w", err)
	}
	if alreadyDeleted {
		// Already gone. Reported as not-found rather than success, so a client
		// deleting a stale list entry is told the list is stale.
		return ErrNotFound
	}

	refs, err := CountReferences(ctx, s.pool, in.ID)
	if err != nil {
		return err
	}
	if refs > 0 {
		return ErrReferenced
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
