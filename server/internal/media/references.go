package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Querier is the subset of pgx satisfied by both a pool and a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TestRef names one published version that uses an asset.
type TestRef struct {
	ID      string
	Title   string
	Version int
}

// References lists the published versions whose questions use the asset, by
// title then version. Any at all blocks deletion with a 409 (§8).
//
// Runs on the caller's querier so it can share the transaction that locked the
// asset, and is served by tvq_media_idx.
func References(ctx context.Context, q Querier, assetID string) ([]TestRef, error) {
	byAsset, err := ReferencesFor(ctx, q, []string{assetID})
	if err != nil {
		return nil, err
	}
	return byAsset[assetID], nil
}

// ReferencesFor is References for a whole page of assets in one query, which
// is what the library list needs: a count and a name per row, without a round
// trip per row.
func ReferencesFor(ctx context.Context, q Querier, assetIDs []string) (map[string][]TestRef, error) {
	rows, err := q.Query(ctx, `
		SELECT DISTINCT tvq.media_asset_id::text, t.id::text, t.title, tv.version
		  FROM app.test_version_questions tvq
		  JOIN app.test_version_sections tvs ON tvs.id = tvq.test_version_section_id
		  JOIN app.test_versions tv ON tv.id = tvs.test_version_id
		  JOIN app.tests t ON t.id = tv.test_id
		 WHERE tvq.media_asset_id = ANY($1::uuid[])
		 ORDER BY tvq.media_asset_id::text, t.title, tv.version, t.id`, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("media: references: %w", err)
	}
	defer rows.Close()
	out := map[string][]TestRef{}
	for rows.Next() {
		var asset string
		var ref TestRef
		if err := rows.Scan(&asset, &ref.ID, &ref.Title, &ref.Version); err != nil {
			return nil, fmt.Errorf("media: references: %w", err)
		}
		out[asset] = append(out[asset], ref)
	}
	return out, rows.Err()
}

// LockForVersionUse takes the row lock that makes the delete check meaningful,
// and must be called by the publish routine before inserting a version question
// that names the asset.
func LockForVersionUse(ctx context.Context, q Querier, assetID string) error {
	var deleted bool
	err := q.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.media_assets WHERE id = $1 FOR UPDATE`,
		assetID).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("media: lock for version use: %w", err)
	}
	if deleted {
		return ErrNotFound
	}
	return nil
}

// ReachableByStudent reports whether a student may mint a signed URL for an
// asset, true only when it is used by a question in a version they have an
// attempt on.
func ReachableByStudent(ctx context.Context, q Querier, studentID, assetID string) (bool, error) {
	var reachable bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		    FROM app.attempts a
		    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
		    JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		   WHERE a.student_id = $1
		     AND q.media_asset_id = $2)`, studentID, assetID).Scan(&reachable)
	if err != nil {
		return false, fmt.Errorf("media: student reachability: %w", err)
	}
	return reachable, nil
}
