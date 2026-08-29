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
//
// SoftDelete locks the asset row and then counts version references, but a
// FOR UPDATE on app.media_assets does not block an INSERT into
// test_version_questions. Without both sides contending on the same row, a
// publish can claim the asset between the count and the update, leaving a
// published version pointing at a soft-deleted asset -- the state the 409
// exists to prevent. The composite foreign key does not cover it either: its
// RESTRICT fires on a real DELETE, and this is a soft one.
//
// Returns ErrNotFound if the asset is gone or already deleted, so a publish
// cannot freeze a reference to a deleted asset either.
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
//
// Still always false: app.attempts arrives in Phase 3, and without it there is
// no attempt to join through. Fail-closed, and correct meanwhile since no
// student can be taking a test yet.
func ReachableByStudent(ctx context.Context, q Querier, studentID, assetID string) (bool, error) {
	_, _, _, _ = ctx, q, studentID, assetID
	return false, nil
}
