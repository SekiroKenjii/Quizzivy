package media

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the subset of pgx satisfied by both a pool and a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// CountReferences reports how many published version questions use the asset.
// A non-zero count blocks deletion.
//
// Always 0 until app.test_version_questions exists in T-2.9. Note that a row
// lock on media_assets will not serialise against a concurrent publish, so the
// publish path must take the same lock when this is implemented.
func CountReferences(ctx context.Context, q Querier, assetID string) (int, error) {
	_, _, _ = ctx, q, assetID
	return 0, nil
}

// ReachableByStudent reports whether a student may mint a signed URL for an
// asset, true only when it is used by a question in a version they have an
// attempt on.
//
// Always false until the version tables exist in T-2.9, which is both
// fail-closed and correct while no test can be published.
func ReachableByStudent(ctx context.Context, q Querier, studentID, assetID string) (bool, error) {
	_, _, _, _ = ctx, q, studentID, assetID
	return false, nil
}
