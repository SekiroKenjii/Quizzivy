package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Querier is the subset of pgx satisfied by both a pool and a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CountReferences reports how many published version questions use the asset.
// A non-zero count blocks deletion with a 409.
//
// Runs on the caller's querier so it can share the transaction that locked the
// asset, and is served by tvq_media_idx.
func CountReferences(ctx context.Context, q Querier, assetID string) (int, error) {
	var n int
	err := q.QueryRow(ctx,
		`SELECT count(*) FROM app.test_version_questions WHERE media_asset_id = $1`,
		assetID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("media: count references: %w", err)
	}
	return n, nil
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
