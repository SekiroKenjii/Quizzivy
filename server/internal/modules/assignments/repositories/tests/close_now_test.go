//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
)

func statusOf(a domain.Assignment) domain.Status {
	return domain.Schedule.StatusAt(time.Now(), a.PublishedAt, a.OpensAt, a.ClosesAt, a.ClosedAt)
}

func TestCloseNowBeforeTheWindowOpensIsRefusedNotAFault(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	scheduled := createFor(t, store, w, windowed(w, time.Hour, 2*time.Hour))
	open := createFor(t, store, w, windowed(w, -time.Hour, time.Hour))
	closed := createFor(t, store, w, windowed(w, -2*time.Hour, -time.Hour))

	for name, c := range map[string]struct {
		id string
		in domain.WriteInput
	}{
		"a scheduled assignment closed now":                      {scheduled.ID, closedNow(windowed(w, time.Hour, 2*time.Hour))},
		"an open assignment closed now with its opening ahead":   {open.ID, closedNow(windowed(w, time.Hour, 2*time.Hour))},
		"a closed assignment closed now with its opening ahead":  {closed.ID, closedNow(windowed(w, time.Hour, 2*time.Hour))},
		"a scheduled assignment closed now with an opening a ms": {scheduled.ID, closedNow(windowed(w, time.Millisecond, 2*time.Hour))},
	} {
		_, err := store.Update(ctx, updating(w, c.id), c.in)
		if message := fieldsOf(t, err)["closeNow"]; message == "" {
			t.Errorf("%s: %v, want a validation error naming closeNow", name, err)
		}
		if errors.Is(err, domain.ErrReopens) {
			t.Errorf("%s: answered as a reopening", name)
		}
	}

	for name, c := range map[string]struct {
		id   string
		want domain.Status
	}{"the scheduled": {scheduled.ID, domain.Scheduled}, "the open": {open.ID, domain.Open}, "the closed": {closed.ID, domain.Closed}} {
		got, err := store.Get(ctx, updating(w, c.id).Scope(), c.id)
		if err != nil {
			t.Fatal(err)
		}
		if got.ClosedAt != nil || statusOf(got) != c.want {
			t.Errorf("%s assignment after the refusals: closedAt %v, status %s, want %s", name, got.ClosedAt, statusOf(got), c.want)
		}
		if actions := auditedAfterCreation(t, pool, c.id); len(actions) != 0 {
			t.Errorf("%s assignment: audit rows after the refusals: %v", name, actions)
		}
	}
}

func TestCloseNowIsAllowedOnceTheWindowHasOpened(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	open := createFor(t, store, w, windowed(w, -time.Hour, time.Hour))
	closing := closedNow(windowed(w, -time.Hour, time.Hour))
	closing.Now = closing.OpensAt
	saved, err := store.Update(ctx, updating(w, open.ID), closing)
	if err != nil {
		t.Fatalf("closing now at the instant the window opens: %v", err)
	}
	if saved.ClosedAt == nil || statusOf(saved) != domain.Closed {
		t.Errorf("closedAt %v, status %s, want it closed", saved.ClosedAt, statusOf(saved))
	}

	another := createFor(t, store, w, windowed(w, -time.Hour, time.Hour))
	if saved, err = store.Update(ctx, updating(w, another.ID), closedNow(windowed(w, -time.Hour, time.Hour))); err != nil || statusOf(saved) != domain.Closed {
		t.Errorf("closing an open assignment now: %v, status %s", err, statusOf(saved))
	}
}

func TestACreateIgnoresCloseNow(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")

	for name, c := range map[string]struct {
		in   domain.WriteInput
		want domain.Status
	}{
		"a window that opens ahead":   {closedNow(windowed(w, time.Hour, 2*time.Hour)), domain.Scheduled},
		"a window that is open":       {closedNow(windowed(w, -time.Hour, time.Hour)), domain.Open},
		"a window that has ended":     {closedNow(windowed(w, -2*time.Hour, -time.Hour)), domain.Closed},
		"a draft with a window ahead": {closedNow(func() domain.WriteInput { in := windowed(w, time.Hour, 2*time.Hour); in.Draft = true; return in }()), domain.Draft},
	} {
		created, err := store.Create(context.Background(), request(w), c.in)
		if err != nil {
			t.Errorf("%s: create with closeNow: %v, want it ignored", name, err)
			continue
		}
		if created.ClosedAt != nil {
			t.Errorf("%s: created with closedAt %v, want none", name, created.ClosedAt)
		}
		if got := statusOf(created); got != c.want {
			t.Errorf("%s: created %s, want %s", name, got, c.want)
		}
	}
}
