package media_test

import (
	"context"
	"testing"
)

// TestReferenceChecksAreWiredOnceVersionTablesExist is a deliberate tripwire.
//
// CountReferences and ReachableByStudent both return a constant because the
// table they must query does not exist yet (migration 00016, T-2.9). Those
// constants are correct today -- nothing can reference an asset and no student
// can have an attempt -- and become security bugs the moment the schema can
// express either relationship: deletes would stop being blocked, and the
// student check would deny access that should be granted.
//
// A stub whose blocker disappears silently is how a permission check quietly
// turns into a no-op. So this fails the moment the table lands, naming what to
// do. It is not testing behaviour; it is refusing to let the two live together.
func TestReferenceChecksAreWiredOnceVersionTablesExist(t *testing.T) {
	pool := newPool(t)

	var exists bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables
		     WHERE table_schema = 'app' AND table_name = 'test_version_questions')`).Scan(&exists)
	if err != nil {
		t.Fatalf("checking for the version content table: %v", err)
	}

	if exists {
		t.Fatal(
			"app.test_version_questions now exists, so media.CountReferences and " +
				"media.ReachableByStudent must stop returning constants:\n" +
				"  - CountReferences: count test_version_questions rows with this media_asset_id,\n" +
				"    so DELETE /admin/media/:id returns 409 for a referenced asset (§8, §15).\n" +
				"  - ReachableByStudent: join attempts -> test_version_questions filtered by the\n" +
				"    caller's own user id, so a student can play their own listening files (§11.2).\n" +
				"Then delete this test and replace it with real coverage of both.")
	}
}
