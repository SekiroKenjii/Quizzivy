package assignments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/assignments"
)

func TestReopeningLiftsAnEarlyCloseAndRecordsWhy(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	created, err := store.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	req := request(w)
	req.ID = created.ID
	closing := legalInput(w)
	closing.CloseNow = true
	if _, err := store.Update(ctx, req, closing); err != nil {
		t.Fatalf("close: %v", err)
	}

	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	reopened, err := store.Reopen(ctx, req, until, "  mất điện cả lớp  ", time.Now())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.ClosedAt != nil {
		t.Errorf("closedAt is still %v after reopening", reopened.ClosedAt)
	}
	if !reopened.ClosesAt.Equal(until) {
		t.Errorf("closesAt = %v, want %v", reopened.ClosesAt, until)
	}
	if got := assignments.StatusAt(time.Now(), reopened.PublishedAt, reopened.OpensAt, reopened.ClosesAt, reopened.ClosedAt); got != assignments.Open {
		t.Errorf("status after reopening: %s, want open", got)
	}

	var reason string
	if err := pool.QueryRow(ctx, `
		SELECT diff->>'reason' FROM app.audit_log
		 WHERE action = 'assignment.reopened' AND entity_id = $1::uuid`, created.ID).Scan(&reason); err != nil {
		t.Fatalf("audit row: %v", err)
	}
	if reason != "mất điện cả lớp" {
		t.Errorf("audited reason %q, want the trimmed one", reason)
	}
}

func TestReopeningRefusesWhatHasNothingToReopen(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	open, err := store.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	req := request(w)
	req.ID = open.ID
	later := time.Now().Add(time.Hour)

	if _, err := store.Reopen(ctx, req, later, "lý do", time.Now()); !errors.Is(err, assignments.ErrNotClosed) {
		t.Errorf("reopening an open assignment returned %v, want ErrNotClosed", err)
	}
	if _, err := store.Reopen(ctx, req, later, "   ", time.Now()); !errors.Is(err, assignments.ErrBlankReason) {
		t.Errorf("a blank reason returned %v, want ErrBlankReason", err)
	}
	if _, err := store.Reopen(ctx, req, time.Now().Add(-time.Minute), "lý do", time.Now()); !errors.Is(err, assignments.ErrClosesInPast) {
		t.Errorf("a past closesAt returned %v, want ErrClosesInPast", err)
	}
	missing := request(w)
	missing.ID = "00000000-0000-7000-8000-000000000000"
	if _, err := store.Reopen(ctx, missing, later, "lý do", time.Now()); !errors.Is(err, assignments.ErrNotFound) {
		t.Errorf("an unknown id returned %v, want ErrNotFound", err)
	}
}

func TestFacetsFollowTheDerivedStatus(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()
	// Other packages create assignments while this counts them, so both
	// readings come from one REPEATABLE READ snapshot; rolling back is the cleanup.
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	store := assignments.NewStore(tx)

	before, err := store.Facets(ctx, assignments.ListInput{})
	if err != nil {
		t.Fatal(err)
	}

	draft := legalInput(w)
	draft.Draft = true
	scheduled := legalInput(w)
	scheduled.OpensAt, scheduled.ClosesAt = time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)
	closed := legalInput(w)
	closed.OpensAt, closed.ClosesAt = time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	for _, in := range []assignments.WriteInput{legalInput(w), draft, scheduled, closed} {
		if _, err := store.Create(ctx, request(w), in); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	after, err := store.Facets(ctx, assignments.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	got := assignments.Facets{
		All: after.All - before.All, Draft: after.Draft - before.Draft,
		Scheduled: after.Scheduled - before.Scheduled, Open: after.Open - before.Open,
		Closed: after.Closed - before.Closed,
	}
	want := assignments.Facets{All: 4, Draft: 1, Scheduled: 1, Open: 1, Closed: 1}
	if got != want {
		t.Errorf("facets moved by %+v, want %+v", got, want)
	}
}
