package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

const retiredFile = `a.purged_at IS NULL AND (a.deleted_at IS NOT NULL OR a.replaced_by IS NOT NULL)`

const namedFile = `(EXISTS (SELECT 1 FROM app.questions r WHERE r.media_asset_id = a.id AND r.deleted_at IS NULL)
	OR EXISTS (SELECT 1 FROM app.group_recordings r WHERE r.media_asset_id = a.id)
	OR EXISTS (SELECT 1 FROM app.group_stimulus_assets r WHERE r.media_asset_id = a.id)
	OR EXISTS (SELECT 1 FROM app.test_version_questions r WHERE r.media_asset_id = a.id)
	OR EXISTS (SELECT 1 FROM app.test_version_group_recordings r WHERE r.media_asset_id = a.id)
	OR EXISTS (SELECT 1 FROM app.test_version_group_assets r WHERE r.media_asset_id = a.id))`

func isNamed(ctx context.Context, q db.Querier, id string) (bool, error) {
	var named bool
	if err := q.QueryRow(ctx, `SELECT `+namedFile+` FROM app.media_assets a WHERE a.id = $1`, id).Scan(&named); err != nil {
		return false, fmt.Errorf("media: is file named: %w", err)
	}
	return named, nil
}

func markRetired(ctx context.Context, tx pgx.Tx, id string, now time.Time) (bool, error) {
	named, err := isNamed(ctx, tx, id)
	if err != nil {
		return false, err
	}
	var unreferencedAt *time.Time
	if !named {
		unreferencedAt = &now
	}
	if _, err := tx.Exec(ctx, `UPDATE app.media_assets SET unreferenced_at = $2 WHERE id = $1`, id, unreferencedAt); err != nil {
		return false, fmt.Errorf("media: mark retired: %w", err)
	}
	return named, nil
}

func describeRetired(ctx context.Context, tx pgx.Tx, asset domain.Asset, usedIn []domain.TestRef, now time.Time) (domain.Retired, error) {
	named, err := markRetired(ctx, tx, asset.ID, now)
	if err != nil {
		return domain.Retired{}, err
	}
	retired := domain.Retired{Asset: asset, UsedIn: usedIn}
	if !named {
		due := now.Add(domain.RetiredFileGrace)
		retired.DeleteAt = &due
	}
	return retired, nil
}

func retiredFigures(ctx context.Context, q db.Querier, scope access.Scope) (held, pending int64, next *time.Time, err error) {
	var since *time.Time
	err = q.QueryRow(ctx, `
		SELECT coalesce(sum(a.bytes) FILTER (WHERE x.named), 0)::bigint,
		       coalesce(sum(a.bytes) FILTER (WHERE NOT x.named), 0)::bigint,
		       min(coalesce(a.unreferenced_at, now())) FILTER (WHERE NOT x.named)
		  FROM app.media_assets a
		 CROSS JOIN LATERAL (SELECT `+namedFile+` AS named) x
		 WHERE `+retiredFile+` AND ($1::boolean OR a.owner_id = $2::uuid)`,
		scope.All, opt.String(scope.UserID)).Scan(&held, &pending, &since)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("media: retired figures: %w", err)
	}
	if since != nil {
		due := since.Add(domain.RetiredFileGrace)
		next = &due
	}
	return held, pending, next, nil
}

func countedBytes(ctx context.Context, q db.Querier, ownerID string) (int64, error) {
	var counted int64
	err := q.QueryRow(ctx, `
		SELECT coalesce(sum(a.bytes), 0)::bigint
		  FROM app.media_assets a
		 WHERE a.owner_id = $1::uuid AND (`+inLibrary+` OR (`+retiredFile+` AND `+namedFile+`))`, ownerID).Scan(&counted)
	if err != nil {
		return 0, fmt.Errorf("media: counted bytes: %w", err)
	}
	return counted, nil
}

// Observe dates the wait of every retired file that nothing names and has no
// date yet, and cancels the wait of every one that something names again. It
// finds the files whose last name went since the last look: a write that leaves
// a file unnamed does not say so, while a write that names it clears the date
// itself (00113).
func (s *Postgres) Observe(ctx context.Context, now time.Time) (domain.Observed, error) {
	var out domain.Observed
	unnamed, err := s.Exec(ctx, `
		UPDATE app.media_assets a SET unreferenced_at = $1
		 WHERE `+retiredFile+` AND a.unreferenced_at IS NULL AND NOT `+namedFile, now)
	if err != nil {
		return out, fmt.Errorf("media: observe unnamed files: %w", err)
	}
	out.Unreferenced = int(unnamed.RowsAffected())
	named, err := s.Exec(ctx, `
		UPDATE app.media_assets a SET unreferenced_at = NULL
		 WHERE `+retiredFile+` AND a.unreferenced_at IS NOT NULL AND `+namedFile)
	if err != nil {
		return out, fmt.Errorf("media: observe named files: %w", err)
	}
	out.Named = int(named.RowsAffected())
	return out, nil
}

// Due lists, in id order after the cursor and at most limit of them, the
// retired files whose wait began at or before due. It takes no lock: Purge
// decides.
func (s *Postgres) Due(ctx context.Context, due time.Time, after string, limit int) ([]string, error) {
	rows, err := s.Query(ctx, `
		SELECT a.id::text FROM app.media_assets a
		 WHERE `+retiredFile+` AND a.unreferenced_at <= $1 AND ($2::uuid IS NULL OR a.id > $2::uuid)
		 ORDER BY a.id LIMIT $3`, due, opt.String(after), limit)
	if err != nil {
		return nil, fmt.Errorf("media: due files: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("media: due files: %w", err)
	}
	return ids, nil
}

// Purge removes one due file's object and marks its row, in one transaction
// that holds the row. The row is taken FOR UPDATE SKIP LOCKED, so a second sweep
// or a writer binding the file leaves it to whoever holds it, and what the
// transaction finds under the lock decides: a file that something names again
// has its wait cancelled, and one that is no longer due is left. The object is
// deleted before the row is marked; a failure of either leaves the file for the
// next sweep, whose delete of an object already gone succeeds.
func (s *Postgres) Purge(ctx context.Context, in domain.PurgeInput) (domain.PurgeOutcome, error) {
	outcome := domain.PurgeSkipped
	err := s.InTx(ctx, "media: purge", func(tx pgx.Tx) error {
		var key, kind string
		var bytes int64
		err := tx.QueryRow(ctx, `
			SELECT a.storage_key, a.kind::text, a.bytes FROM app.media_assets a
			 WHERE a.id = $1 AND `+retiredFile+` AND a.unreferenced_at <= $2
			 FOR UPDATE SKIP LOCKED`, in.ID, in.Due).Scan(&key, &kind, &bytes)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("media: lock retired file: %w", err)
		}
		named, err := isNamed(ctx, tx, in.ID)
		if err != nil {
			return err
		}
		if named {
			outcome = domain.PurgeNamed
			_, err := tx.Exec(ctx, `UPDATE app.media_assets SET unreferenced_at = NULL WHERE id = $1`, in.ID)
			return err
		}
		if err := in.Remove(ctx, key); err != nil {
			return fmt.Errorf("media: remove object: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE app.media_assets SET purged_at = $2 WHERE id = $1`, in.ID, in.Now); err != nil {
			return fmt.Errorf("media: mark purged: %w", err)
		}
		diff, err := json.Marshal(map[string]any{"kind": kind, "bytes": bytes})
		if err != nil {
			return err
		}
		outcome = domain.PurgeRemoved
		return audit.Write(ctx, tx, audit.Entry{Action: "media.purged", Entity: auditedEntity, EntityID: &in.ID, OccurredAt: in.Now, Diff: diff})
	})
	if err != nil {
		return domain.PurgeSkipped, err
	}
	return outcome, nil
}
