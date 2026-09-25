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

func TestNextDueReportsLeasesAndRetiringVersions(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	run := h.schedule(t, version, 3)
	h.claim(t, policy(version))

	lease := time.Now().Add(2 * time.Minute).UTC().Truncate(time.Microsecond)
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_runs SET lease_until=$2 WHERE id=$1`, run.ID, lease); err != nil {
		t.Fatal(err)
	}
	if due, pending, err := h.repo.NextDue(ctx, version); err != nil || !pending || !due.Equal(lease) {
		t.Fatalf("a running lease falls due at %v, got %v %v %v", lease, due, pending, err)
	}

	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_runs SET attempt_count=max_attempts WHERE id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	if due, pending, err := h.repo.NextDue(ctx, version); err != nil || !pending || !due.Equal(lease) {
		t.Fatalf("an exhausted lease still falls due to be expired at %v, got %v %v %v", lease, due, pending, err)
	}

	retiring := lease.Add(10 * time.Minute)
	due, pending, err := h.repo.NextDue(ctx, uuid.NewString())
	if err != nil || !pending || due.After(retiring) || due.Before(retiring.Add(-time.Minute)) {
		t.Fatalf("another version's lease retires at %v, got %v %v %v", retiring, due, pending, err)
	}

	if _, err := h.pool.Exec(ctx, `UPDATE app.word_imports SET status='needs_review' WHERE id=(SELECT import_id FROM app.word_import_runs WHERE id=$1)`, run.ID); err != nil {
		t.Fatal(err)
	}
	due, pending, err = h.repo.NextDue(ctx, version)
	if err != nil || (pending && due.Before(time.Now().Add(9*time.Minute))) {
		t.Fatalf("a run whose import left processing still counts as due: %v %v %v", due, pending, err)
	}
}
