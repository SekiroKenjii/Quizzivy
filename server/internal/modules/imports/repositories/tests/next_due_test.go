//go:build integration

package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNextDueReportsOnlyWorkClaimWouldTake(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	run := h.schedule(t, version, 3)

	due, pending, err := h.repo.NextDue(ctx, version)
	if err != nil || !pending || due.After(time.Now()) {
		t.Fatalf("a queued run is due now: %v %v %v", due, pending, err)
	}

	later := time.Now().Add(3 * time.Minute).UTC().Truncate(time.Microsecond)
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_runs SET available_at=$2 WHERE id=$1`, run.ID, later); err != nil {
		t.Fatal(err)
	}
	due, pending, err = h.repo.NextDue(ctx, version)
	if err != nil || !pending || !due.Equal(later) {
		t.Fatalf("a delayed retry is due at %v, got %v %v %v", later, due, pending, err)
	}

	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_runs SET attempt_count=max_attempts WHERE id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	due, pending, err = h.repo.NextDue(ctx, version)
	if err != nil || (pending && due.Before(time.Now().Add(9*time.Minute))) {
		t.Fatalf("a run Claim would never take still counts as due: %v %v %v", due, pending, err)
	}
}
