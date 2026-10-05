//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func publishedFixture(t *testing.T) (*snapshotFixture, publishOwnedIDs) {
	t.Helper()
	fixture := newSnapshotFixture(t)
	b := fixture.builder
	question := b.shortAnswer("Lifecycle snapshot", "2.00")
	draft := b.draft("Lifecycle publication", question)
	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("lifecycle publish: %v", err)
	}
	ctx := context.Background()
	var role string
	if err := fixture.conn.QueryRow(ctx, `
		SELECT r.builtin_key FROM app.users u JOIN app.roles r ON r.id = u.role_id WHERE u.id = $1`, fixture.author).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "teacher" {
		t.Fatalf("snapshot author role %q, want teacher", role)
	}
	var auditID string
	if err := fixture.conn.QueryRow(ctx, `
		SELECT id::text FROM app.audit_log
		 WHERE actor_user_id = $1 AND action = 'test.published' AND entity = 'test_version' AND entity_id = $2`, fixture.author, version.ID).Scan(&auditID); err != nil {
		t.Fatalf("genuine publication audit before rollback: %v", err)
	}
	owned, err := capturePublishOwned(ctx, fixture.conn, fixture.author)
	if err != nil {
		t.Fatal(err)
	}
	if len(owned.Questions) != 1 || owned.Questions[0] != question || len(owned.Tests) != 1 || owned.Tests[0] != draft.ID || len(owned.Versions) != 1 || owned.Versions[0] != version.ID {
		t.Fatalf("captured publication identities disagree: %+v", owned)
	}
	found := false
	for _, id := range owned.Audit {
		found = found || id == auditID
	}
	if !found {
		t.Fatal("captured audit identities omit the actual publication audit")
	}
	t.Logf("genuine publication before rollback: author=%s question=%s test=%s version=%s audit=%s", fixture.author, question, draft.ID, version.ID, auditID)
	return fixture, owned
}

func TestPublishFixtureRollsBackOwnedPublication(t *testing.T) {
	observer := newPool(t)
	var owned publishOwnedIDs
	if !t.Run("published", func(t *testing.T) {
		_, owned = publishedFixture(t)
	}) {
		return
	}
	assertPublishOwnedAbsent(t, observer, owned)
}

func TestPublishFixtureCleansUpAfterFatal(t *testing.T) {
	const probe = "QUIZZIVY_PUBLISH_FIXTURE_FATAL_PROBE"
	if path := os.Getenv(probe); path != "" {
		_, owned := publishedFixture(t)
		raw, err := json.Marshal(owned)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		t.Fatal("intentional publish fixture fatal after owned publication")
	}
	observer := newPool(t)
	path := filepath.Join(t.TempDir(), "owned.json")
	child := exec.Command(os.Args[0], "-test.run=^TestPublishFixtureCleansUpAfterFatal$", "-test.v")
	child.Env = append(os.Environ(), probe+"="+path)
	output, err := child.CombinedOutput()
	if err == nil {
		t.Fatal("intentional fatal child unexpectedly passed")
	}
	failure, ok := err.(*exec.ExitError)
	if !ok || failure.ExitCode() != 1 {
		t.Fatalf("intentional child did not fail as a test: %v", err)
	}
	log := string(output)
	t.Logf("intentional child exit1 (retained real failure):\n%s", log)
	if !strings.Contains(log, "intentional publish fixture fatal after owned publication") || !strings.Contains(log, "snapshot fixture rollback verified:") {
		t.Fatal("child omitted actual fatal or completed rollback evidence")
	}
	if strings.Contains(log, "snapshot fixture capture:") || strings.Contains(log, "snapshot fixture rollback:") || strings.Contains(log, "retained") {
		t.Fatal("child cleanup reported a failure")
	}
	if !publishFatalChildVerified(log) {
		t.Fatal("child publication cleanup was not verified")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var owned publishOwnedIDs
	if err := json.Unmarshal(raw, &owned); err != nil {
		t.Fatal(err)
	}
	if owned.Author == "" || len(owned.Questions) != 1 || len(owned.Tests) != 1 || len(owned.Versions) != 1 || len(owned.Audit) == 0 {
		t.Fatalf("intentional child did not capture the real owned publication: %+v", owned)
	}
	assertPublishOwnedAbsent(t, observer, owned)
}

func publishFatalChildVerified(log string) bool {
	return strings.Contains(log, "intentional publish fixture fatal after owned publication") &&
		strings.Contains(log, "snapshot fixture rollback verified:") &&
		!strings.Contains(log, "snapshot fixture capture:") &&
		!strings.Contains(log, "snapshot fixture rollback:") &&
		!strings.Contains(log, " absence:") &&
		!strings.Contains(log, "retained")
}
