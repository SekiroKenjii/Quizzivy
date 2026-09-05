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

// ReferencedError is ErrReferenced carrying the versions that block the
// delete, so the refusal can name them (A-07). errors.Is(err, ErrReferenced)
// still holds.
type ReferencedError struct{ Tests []TestRef }

func (e *ReferencedError) Error() string        { return ErrReferenced.Error() }
func (e *ReferencedError) Is(target error) bool { return target == ErrReferenced }

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
		return ErrNotFound
	}
	refs, err := References(ctx, tx, in.ID)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return &ReferencedError{Tests: refs}
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
