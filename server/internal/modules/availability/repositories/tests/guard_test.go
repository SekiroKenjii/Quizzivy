//go:build integration

package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/availability/repositories"
	"quizzivy/internal/platform/db"
)

type world struct {
	ctx context.Context
	tx  pgx.Tx
	now time.Time
}

func isolated(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	return world{ctx: ctx, tx: tx, now: now.Add(3650 * 24 * time.Hour).Truncate(time.Second)}
}

func (w world) window(t *testing.T, startsAt, endsAt time.Time, cancelled bool) {
	t.Helper()
	if _, err := w.tx.Exec(w.ctx, `
		INSERT INTO app.maintenance_windows (starts_at, ends_at, cancelled_at)
		VALUES ($1, $2, CASE WHEN $3 THEN now() END)`, startsAt, endsAt, cancelled); err != nil {
		t.Fatal(err)
	}
}

func TestTheGuardFindsAWindowTheAttemptWouldRunInto(t *testing.T) {
	w := isolated(t)
	w.window(t, w.now.Add(30*time.Minute), w.now.Add(90*time.Minute), false)

	got, err := repositories.GuardAttemptStart(w.ctx, w.tx, w.now, w.now.Add(45*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.StartsAt.Equal(w.now.Add(30*time.Minute)) {
		t.Fatalf("guard = %+v, want the window starting in thirty minutes", got)
	}
}

func TestTheGuardFindsAWindowAlreadyUnderWay(t *testing.T) {
	w := isolated(t)
	w.window(t, w.now.Add(-10*time.Minute), w.now.Add(time.Hour), false)

	got, err := repositories.GuardAttemptStart(w.ctx, w.tx, w.now, w.now.Add(45*time.Minute))
	if err != nil || got == nil {
		t.Fatalf("guard = %+v, %v; want the running window", got, err)
	}
}

func TestTheGuardLetsThroughWhatItDoesNotOverlap(t *testing.T) {
	for name, place := range map[string]func(w world, t *testing.T){
		"a window starting as the attempt ends": func(w world, t *testing.T) {
			w.window(t, w.now.Add(45*time.Minute), w.now.Add(2*time.Hour), false)
		},
		"a window that ended as the attempt starts": func(w world, t *testing.T) {
			w.window(t, w.now.Add(-time.Hour), w.now, false)
		},
		"a past window": func(w world, t *testing.T) {
			w.window(t, w.now.Add(-3*time.Hour), w.now.Add(-2*time.Hour), false)
		},
		"a cancelled window": func(w world, t *testing.T) {
			w.window(t, w.now.Add(10*time.Minute), w.now.Add(time.Hour), true)
		},
		"a window after the attempt": func(w world, t *testing.T) {
			w.window(t, w.now.Add(2*time.Hour), w.now.Add(3*time.Hour), false)
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := isolated(t)
			place(w, t)
			got, err := repositories.GuardAttemptStart(w.ctx, w.tx, w.now, w.now.Add(45*time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if got != nil {
				t.Errorf("guard = %+v, want nothing", got)
			}
		})
	}
}

func TestNextIsTheEarliestWindowThatHasNotEnded(t *testing.T) {
	w := isolated(t)
	w.window(t, w.now.Add(-3*time.Hour), w.now.Add(-2*time.Hour), false)
	w.window(t, w.now.Add(time.Hour), w.now.Add(2*time.Hour), true)
	w.window(t, w.now.Add(3*time.Hour), w.now.Add(4*time.Hour), false)
	w.window(t, w.now.Add(5*time.Hour), w.now.Add(6*time.Hour), false)

	got, err := repositories.NewPostgres(db.NewContext(w.tx)).Next(w.ctx, w.now)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.StartsAt.Equal(w.now.Add(3*time.Hour)) {
		t.Errorf("next = %+v, want the one starting in three hours", got)
	}
}
