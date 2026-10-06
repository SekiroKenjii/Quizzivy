//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"quizzivy/internal/platform/db"
)

type transientPublishObserver struct {
	db.Querier
	t      *testing.T
	failed bool
}

func (q *transientPublishObserver) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if !q.failed {
		q.failed = true
		q.t.Log("controlled transient observer query error injected once")
		return publishProbeErrorRow{}
	}
	return q.Querier.QueryRow(ctx, sql, args...)
}

type publishProbeErrorRow struct{}

func (publishProbeErrorRow) Scan(...any) error {
	return errors.New("controlled transient publish absence query failure")
}

func TestPublishFixtureRejectsTransientAbsenceProbeFailure(t *testing.T) {
	const probe = "QUIZZIVY_PUBLISH_FIXTURE_ABSENCE_PROBE"
	if path := os.Getenv(probe); path != "" {
		fixture, owned := publishedFixture(t)
		fixture.observer = &transientPublishObserver{Querier: fixture.observer, t: t}
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
	child := exec.Command(os.Args[0], "-test.run=^TestPublishFixtureRejectsTransientAbsenceProbeFailure$", "-test.v")
	child.Env = append(os.Environ(), probe+"="+path)
	output, err := child.CombinedOutput()
	failure, ok := err.(*exec.ExitError)
	if !ok || failure.ExitCode() != 1 {
		t.Fatalf("probe child must actually fail exit1: %v", err)
	}
	log := string(output)
	t.Logf("controlled probe child exit1 (retained fatal and query failure):\n%s", log)
	if !strings.Contains(log, "intentional publish fixture fatal after owned publication") || !strings.Contains(log, "snapshot fixture users absence: controlled transient publish absence query failure") || strings.Count(log, "controlled transient observer query error injected once") != 1 {
		t.Fatal("child omitted genuine fatal or single injected absence-query failure")
	}
	if publishFatalChildVerified(log) {
		t.Error("fatal child verification accepted an unrelated absence-query failure")
	}
	if strings.Contains(log, "snapshot fixture rollback verified:") {
		t.Error("absence-query failure still emitted verified rollback marker")
	}
	if strings.Contains(log, "snapshot fixture capture:") || strings.Contains(log, "snapshot fixture rollback:") || strings.Contains(log, "retained") {
		t.Error("child has an additional unrelated cleanup failure")
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
		t.Fatalf("probe child did not capture genuine public publication: %+v", owned)
	}
	assertPublishOwnedAbsent(t, observer, owned)
}
