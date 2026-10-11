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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func windowed(w world, opens, closes time.Duration) domain.WriteInput {
	in := legalInput(w)
	in.OpensAt, in.ClosesAt = in.Now.Add(opens), in.Now.Add(closes)
	return in
}

func shuffled(in domain.WriteInput) domain.WriteInput {
	in.ShuffleQ = true
	return in
}

func closedNow(in domain.WriteInput) domain.WriteInput {
	in.CloseNow = true
	return in
}

func auditedAfterCreation(t *testing.T, pool *pgxpool.Pool, assignmentID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT action FROM app.audit_log WHERE entity = 'assignment' AND entity_id = $1::uuid AND action <> 'assignment.created' ORDER BY id`,
		assignmentID)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return actions
}

func TestAWindowThatWouldOpenAClosedAssignmentAgainIsRefusedAndWritesNothing(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	closed := createFor(t, store, w, windowed(w, -2*time.Hour, -time.Hour))
	req := updating(w, closed.ID)

	for name, in := range map[string]domain.WriteInput{
		"a later close":                      windowed(w, -2*time.Hour, time.Hour),
		"a close a minute ahead":             windowed(w, -2*time.Hour, time.Minute),
		"a window that opens later":          windowed(w, time.Hour, 2*time.Hour),
		"a later close with other edits too": shuffled(windowed(w, -2*time.Hour, time.Hour)),
	} {
		if _, err := store.Update(ctx, req, in); !errors.Is(err, domain.ErrReopens) {
			t.Errorf("%s: %v, want ErrReopens", name, err)
		}
	}

	got, err := store.Get(ctx, req.Scope(), closed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OpensAt.Equal(closed.OpensAt) || !got.ClosesAt.Equal(closed.ClosesAt) || got.ShuffleQ {
		t.Errorf("a refused update was kept: opens %v closes %v shuffle %v", got.OpensAt, got.ClosesAt, got.ShuffleQ)
	}
	if status := domain.Schedule.StatusAt(time.Now(), got.PublishedAt, got.OpensAt, got.ClosesAt, got.ClosedAt); status != domain.Closed {
		t.Errorf("the assignment is %s after the refusals, want closed", status)
	}
	if actions := auditedAfterCreation(t, pool, closed.ID); len(actions) != 0 {
		t.Errorf("audit rows after refused updates: %v, want only the creation", actions)
	}

	until := time.Now().Add(time.Hour).Truncate(time.Second)
	reopened, err := store.Reopen(ctx, req, until, "học viên mất điện", time.Now())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !reopened.ClosesAt.Equal(until) {
		t.Errorf("reopened to %v, want %v", reopened.ClosesAt, until)
	}
	if actions := auditedAfterCreation(t, pool, closed.ID); len(actions) != 1 || actions[0] != "assignment.reopened" {
		t.Errorf("audit rows after reopening: %v, want the one with the reason", actions)
	}
}

func TestAClosedAssignmentKeepsEverythingThatDoesNotOpenItAgain(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	closed := createFor(t, store, w, windowed(w, -2*time.Hour, -time.Hour))
	req := updating(w, closed.ID)

	edited := shuffled(legalInput(w))
	edited.OpensAt, edited.ClosesAt = closed.OpensAt, closed.ClosesAt
	edited.DurationMin, edited.MaxAttempts = 90, 3
	edited.Review.ShowScore = false
	saved, err := store.Update(ctx, req, edited)
	if err != nil {
		t.Fatalf("everything else with the stored window: %v", err)
	}
	if !saved.ShuffleQ || saved.DurationMin != 90 || saved.MaxAttempts != 3 || saved.Review.ShowScore {
		t.Errorf("the edits were not saved: %+v", saved)
	}

	for name, in := range map[string]domain.WriteInput{
		"an earlier close":                  windowed(w, -2*time.Hour, -90*time.Minute),
		"an earlier window":                 windowed(w, -3*time.Hour, -2*time.Hour),
		"a close that is still in the past": windowed(w, -2*time.Hour, -time.Second),
		"closing now beside a window ahead": closedNow(windowed(w, -2*time.Hour, time.Hour)),
	} {
		in.DurationMin, in.MaxAttempts = 90, 3
		if _, err := store.Update(ctx, req, in); err != nil {
			t.Errorf("%s: %v, want it saved", name, err)
		}
	}

	got, err := store.Get(ctx, req.Scope(), closed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status := domain.Schedule.StatusAt(time.Now(), got.PublishedAt, got.OpensAt, got.ClosesAt, got.ClosedAt); status != domain.Closed {
		t.Errorf("the assignment is %s after those saves, want closed", status)
	}
}

func TestAnAssignmentClosedEarlyStaysClosedWhateverWindowIsSaved(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	open := createFor(t, store, w, windowed(w, -time.Hour, time.Hour))
	req := updating(w, open.ID)
	if _, err := store.Update(ctx, req, closedNow(windowed(w, -time.Hour, time.Hour))); err != nil {
		t.Fatalf("close early: %v", err)
	}

	for name, in := range map[string]domain.WriteInput{
		"a later close":    windowed(w, -time.Hour, 3*time.Hour),
		"an earlier close": windowed(w, -2*time.Hour, -time.Minute),
	} {
		got, err := store.Update(ctx, req, in)
		if err != nil {
			t.Errorf("%s: %v, want it saved", name, err)
			continue
		}
		if got.ClosedAt == nil {
			t.Errorf("%s: the early close was lifted", name)
		}
		if status := domain.Schedule.StatusAt(time.Now(), got.PublishedAt, got.OpensAt, got.ClosesAt, got.ClosedAt); status != domain.Closed {
			t.Errorf("%s: the assignment is %s, want closed", name, status)
		}
	}

	if _, err := store.Update(ctx, req, windowed(w, time.Hour, 3*time.Hour)); !errors.Is(err, domain.ErrReopens) {
		t.Errorf("a window that opens after the early close: %v, want ErrReopens", err)
	}
}

func TestTheGuardFlipsExactlyAtTheClose(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	opens := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	closes := opens.Add(time.Hour)
	base := legalInput(w)
	base.OpensAt, base.ClosesAt, base.Now = opens, closes, opens.Add(-time.Hour)

	for _, c := range []struct {
		when    string
		now     time.Time
		refused bool
	}{
		{"a microsecond before it closes", closes.Add(-time.Microsecond), false},
		{"the instant it closes", closes, true},
		{"a microsecond after it closes", closes.Add(time.Microsecond), true},
	} {
		created := createFor(t, store, w, base)
		later := base
		later.Now, later.ClosesAt = c.now, closes.Add(time.Hour)
		_, err := store.Update(ctx, updating(w, created.ID), later)
		if refused := errors.Is(err, domain.ErrReopens); refused != c.refused || (!c.refused && err != nil) {
			t.Errorf("%s: refused=%v err=%v, want refused=%v", c.when, refused, err, c.refused)
		}
	}
}
