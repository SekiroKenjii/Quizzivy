package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
)

const scopedAsset = `($2::boolean OR owner_id = $3::uuid)`

// SoftDelete marks an unreferenced asset in the actor's scope deleted and
// audits it. The scope is checked before the references, so an asset outside
// it answers ErrNotFound and never the tests and groups that use it. It answers
// the file as it left the library, with the time its object goes: seven days on
// when nothing names it, and none while a live question still does.
func (s *Postgres) SoftDelete(ctx context.Context, in domain.DeleteInput) (domain.Retired, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Retired{}, fmt.Errorf("media: begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var gone bool
	err = tx.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL OR purged_at IS NOT NULL FROM app.media_assets WHERE id = $1 AND `+scopedAsset+` FOR UPDATE`,
		in.ID, in.All, opt.String(in.ActorID)).Scan(&gone)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Retired{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Retired{}, fmt.Errorf("media: lock asset: %w", err)
	}
	if gone {
		return domain.Retired{}, domain.ErrNotFound
	}
	refs, err := References(ctx, tx, in.ID)
	if err != nil {
		return domain.Retired{}, err
	}
	groups, err := GroupReferences(ctx, tx, in.ID)
	if err != nil {
		return domain.Retired{}, err
	}
	if len(refs) > 0 || len(groups) > 0 {
		return domain.Retired{}, &domain.ReferencedError{Tests: refs, Groups: groups}
	}

	asset, err := scanAsset(tx.QueryRow(ctx,
		`UPDATE app.media_assets AS a SET deleted_at = $2 WHERE a.id = $1 RETURNING `+assetColumns,
		in.ID, in.Now))
	if err != nil {
		return domain.Retired{}, fmt.Errorf("media: soft delete: %w", err)
	}
	retired, err := describeRetired(ctx, tx, asset, refs, in.Now)
	if err != nil {
		return domain.Retired{}, err
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "media.deleted",
		Entity:      "media_asset",
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          opt.String(in.IP),
		UserAgent:   opt.String(in.UserAgent),
	}); err != nil {
		return domain.Retired{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Retired{}, fmt.Errorf("media: commit delete: %w", err)
	}
	return retired, nil
}
