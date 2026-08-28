package media

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the slice of pgx both a pool and a transaction satisfy.
//
// It exists so these checks can run INSIDE the caller's transaction. The
// earlier signature took a *pgxpool.Pool, which meant CountReferences ran on a
// different connection and a different snapshot from the SoftDelete transaction
// that had just taken `FOR UPDATE` on the asset -- and, worse, made it
// impossible to fix at the call site. Whoever filled the stub in would see the
// call sitting between a row lock and an UPDATE and reasonably conclude it was
// covered by the transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// This file answers the two questions T-2.4 needs about an asset's
// relationships, and neither can be answered properly yet.
//
// Both are decided against `test_version_questions`, which migration 00016
// creates in T-2.9. Until then there is no table that can hold a reference to a
// media asset and no table that can hold an attempt, so the true answers today
// are "nothing references it" and "no student can reach it" -- which is what
// these return. They are correct now and will be wrong the moment those tables
// land, so TestReferenceChecksAreWiredOnceVersionTablesExist fails as soon as
// the schema can support the real queries. That is the point of it: a stub that
// stays silent after its blocker is gone is how a permission check quietly
// becomes a no-op.
//
// The direction of each stub is chosen so that being wrong is safe:
// ReachableByStudent denies, rather than granting access it cannot verify.

// versionContentTable is the table both real implementations will query. Named
// once so the guard test and these comments cannot drift apart.
const versionContentTable = "test_version_questions"

// CountReferences reports how many published version questions use this asset.
// A non-zero count blocks deletion (§8, §15).
//
// Returns 0 unconditionally: `test_version_questions` does not exist yet, so no
// reference can exist. When it does, this becomes
//
//	SELECT count(*) FROM app.test_version_questions
//	 WHERE media_asset_id = $1
//
// which `tvq_media_idx` serves.
//
// NOT ENOUGH ON ITS OWN, and worth knowing before T-2.9. Running this inside
// SoftDelete's transaction makes it see that transaction's snapshot, but a
// `FOR UPDATE` on a media_assets row does not block an INSERT into
// test_version_questions -- different table, no lock conflict. Serialising the
// two properly means the PUBLISH path taking the same asset row lock, so the
// two operations contend on one row. Until it does, the 409 narrows the window
// rather than closing it.
func CountReferences(ctx context.Context, q Querier, assetID string) (int, error) {
	_, _, _ = ctx, q, assetID
	return 0, nil
}

// ReachableByStudent reports whether a student may mint a signed URL for an
// asset -- true only when the asset is used by a question in a version the
// student has an attempt on (§11.2). Checked against the version content, never
// by asset id alone: any student knowing any asset id could otherwise read every
// listening file in the school.
//
// Returns false unconditionally, which is both fail-closed and, today, exactly
// right: no test can be published and no attempt can exist, so no asset is
// reachable by anyone. When the tables land this becomes a join from `attempts`
// through `test_version_questions` filtered by the caller's own user id.
func ReachableByStudent(ctx context.Context, q Querier, studentID, assetID string) (bool, error) {
	_, _, _, _ = ctx, q, studentID, assetID
	return false, nil
}
