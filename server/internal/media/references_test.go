package media_test

import (
	"context"
	"testing"
)

// TestStudentReachabilityIsWiredOnceAttemptsExist keeps the second half of the
// stub honest.
//
// media.CountReferences is now real -- test_version_questions arrived in T-2.9.
// ReachableByStudent is not, because it joins through app.attempts, which
// Phase 3 creates. The original tripwire named both and fired on the version
// tables, which is one table too early for this half; this is the same guard
// re-pointed at the blocker that actually remains.
func TestStudentReachabilityIsWiredOnceAttemptsExist(t *testing.T) {
	pool := newPool(t)

	var exists bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables
		     WHERE table_schema = 'app' AND table_name = 'attempts')`).Scan(&exists)
	if err != nil {
		t.Fatalf("checking for the attempts table: %v", err)
	}

	if exists {
		t.Fatal(
			"app.attempts now exists, so media.ReachableByStudent must stop returning false:\n" +
				"  join attempts -> test_versions -> test_version_sections ->\n" +
				"  test_version_questions, filtered by the caller's own user id, so a student\n" +
				"  can play the listening files of a test they are actually sitting (§11.2).\n" +
				"Then delete this test and replace it with real coverage.")
	}
}
