//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/schedule"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (w *reachWorld) updateWindow(t *testing.T, closesAt time.Time) domain.WriteInput {
	t.Helper()
	in := input(w.versionA, []string{w.classA}, nil)
	in.OpensAt, in.ClosesAt, in.DurationMin = time.Now().Add(-time.Hour), closesAt, 45
	return in
}

func (w *reachWorld) auditedMoves(t *testing.T, attempt string) (count int, reason, actor string) {
	t.Helper()
	if err := w.tx.QueryRow(context.Background(), `
		SELECT count(*), coalesce(min(diff->>'reason'), ''), coalesce(min(actor_user_id::text), '')
		  FROM app.audit_log WHERE action = 'attempt.extended' AND entity_id = $1::uuid`, attempt).Scan(&count, &reason, &actor); err != nil {
		t.Fatal(err)
	}
	return count, reason, actor
}

func TestLengtheningTheCloseThroughUpdateMovesAnAttemptInProgress(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	cutStudent, roomyStudent, deliveredStudent := w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a)
	closes := w.closesAt(t, assignment)
	cut := w.sitting(t, assignment, cutStudent, 5*time.Minute, closes, "in_progress")
	roomy := w.sitting(t, assignment, roomyStudent, 30*time.Minute, time.Now().Add(15*time.Minute), "in_progress")
	delivered := w.sitting(t, assignment, deliveredStudent, 5*time.Minute, closes, "submitted")
	roomyBefore := w.deadlineOf(t, roomy)

	req := as(w.a, false)
	req.ID = assignment
	req.IP, req.UserAgent = "203.0.113.7", "go-test"
	if _, err := w.store.Update(ctx, req, w.updateWindow(t, closes.Add(2*time.Hour))); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got, want := w.deadlineOf(t, cut), w.startedOf(t, cut).Add(45*time.Minute); !got.Equal(want) {
		t.Errorf("the attempt the close held down has deadline %v after the close moved later, want its start plus 45 minutes, %v", got, want)
	}
	if got := w.deadlineOf(t, roomy); !got.Equal(roomyBefore) {
		t.Errorf("an attempt whose time limit set its deadline moved to %v from %v", got, roomyBefore)
	}
	if got := w.deadlineOf(t, delivered); !got.Equal(closes) {
		t.Errorf("an attempt already handed in moved to %v from %v", got, closes)
	}
	if n, reason, actor := w.auditedMoves(t, cut); n != 1 || reason != "assignment_updated" || actor != w.a {
		t.Errorf("the moved attempt's audit = %d rows, reason %q, by %s; want one by the teacher for assignment_updated", n, reason, actor)
	}
	if n, _, _ := w.auditedMoves(t, roomy); n != 0 {
		t.Errorf("%d audit entries for an attempt that did not move", n)
	}
}

func TestShorteningTheCloseOrClosingNowThroughUpdateMovesNoAttempt(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 90*time.Minute, 120)
	student := w.classmate(t, w.classA, w.a)
	closes := w.closesAt(t, assignment)
	attempt := w.sitting(t, assignment, student, 5*time.Minute, closes, "in_progress")

	req := as(w.a, false)
	req.ID = assignment
	shorter := w.updateWindow(t, closes.Add(-30*time.Minute))
	shorter.DurationMin = 120
	if _, err := w.store.Update(ctx, req, shorter); err != nil {
		t.Fatalf("shorten: %v", err)
	}
	if got := w.deadlineOf(t, attempt); !got.Equal(closes) {
		t.Errorf("a shorter close moved the deadline to %v from %v", got, closes)
	}

	closing := w.updateWindow(t, closes.Add(-30*time.Minute))
	closing.DurationMin, closing.CloseNow = 120, true
	if _, err := w.store.Update(ctx, req, closing); err != nil {
		t.Fatalf("close now: %v", err)
	}
	if got := w.deadlineOf(t, attempt); !got.Equal(closes) {
		t.Errorf("closing early moved the deadline to %v from %v", got, closes)
	}
	if n, _, _ := w.auditedMoves(t, attempt); n != 0 {
		t.Errorf("%d audit entries for a window that never let the attempt run longer", n)
	}
}

func TestReopeningAfterAnEarlyCloseMovesAnAttemptInProgress(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	assignment := w.closingSoon(t, w.a, w.classA, 20*time.Minute, 45)
	student, bystander := w.classmate(t, w.classA, w.a), w.classmate(t, w.classA, w.a)
	closes := w.closesAt(t, assignment)
	running := w.sitting(t, assignment, student, 5*time.Minute, closes, "in_progress")
	delivered := w.sitting(t, assignment, bystander, 5*time.Minute, closes, "submitted")
	if _, err := w.tx.Exec(ctx, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1::uuid`, assignment); err != nil {
		t.Fatal(err)
	}

	req := as(w.a, false)
	req.ID = assignment
	req.IP, req.UserAgent = "203.0.113.7", "go-test"
	if _, err := w.store.Reopen(ctx, req, closes.Add(3*time.Hour), "mất điện", time.Now()); err != nil {
		t.Fatalf("reopen: %v", err)
	}

	if got, want := w.deadlineOf(t, running), w.startedOf(t, running).Add(45*time.Minute); !got.Equal(want) {
		t.Errorf("the attempt still running from before the early close has deadline %v, want its start plus 45 minutes, %v", got, want)
	}
	if got := w.deadlineOf(t, delivered); !got.Equal(closes) {
		t.Errorf("an attempt already handed in moved to %v from %v", got, closes)
	}
	if n, reason, actor := w.auditedMoves(t, running); n != 1 || reason != "assignment_reopened" || actor != w.a {
		t.Errorf("the moved attempt's audit = %d rows, reason %q, by %s; want one by the teacher for assignment_reopened", n, reason, actor)
	}
}

func poolNamed(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = name
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func waitingOnTheAdvisoryLock(t *testing.T, pool *pgxpool.Pool, application string) bool {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			 WHERE application_name = $1 AND wait_event_type = 'Lock' AND wait_event = 'advisory'`, application).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestEveryWriterOfAnAssignmentsWindowWaitsForTheMaintenanceLock(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()
	created, err := repositories.NewPostgres(db.NewContext(pool)).Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatal(err)
	}
	req := request(w)
	req.ID = created.ID

	writers := []struct {
		name    string
		prepare func() error
		run     func(store *repositories.Postgres) error
	}{
		{name: "Update", run: func(s *repositories.Postgres) error {
			_, err := s.Update(ctx, req, legalInput(w))
			return err
		}},
		{name: "Extend", run: func(s *repositories.Postgres) error {
			_, err := s.Extend(ctx, req, 10, false, time.Now())
			return err
		}},
		{name: "SetOverrides", run: func(s *repositories.Postgres) error {
			_, err := s.SetOverrides(ctx, req, domain.OverrideInput{StudentIDs: []string{w.student}, ExtraAttempts: ptr(1), Reason: "khóa", Now: time.Now()})
			return err
		}},
		{name: "DeleteOverride", run: func(s *repositories.Postgres) error {
			return s.DeleteOverride(ctx, req, w.student, time.Now())
		}},
		{name: "Reopen", prepare: func() error {
			_, err := pool.Exec(ctx, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1::uuid`, created.ID)
			return err
		}, run: func(s *repositories.Postgres) error {
			_, err := s.Reopen(ctx, req, time.Now().Add(2*time.Hour), "khóa", time.Now())
			return err
		}},
	}

	for _, writer := range writers {
		if writer.prepare != nil {
			if err := writer.prepare(); err != nil {
				t.Fatalf("%s: prepare: %v", writer.name, err)
			}
		}
		application := "qv-window-writer-" + writer.name + "-" + nonce(t)
		store := repositories.NewPostgres(db.NewContext(poolNamed(t, application)))

		holder, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := holder.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				t.Errorf("%s: releasing the maintenance lock: %v", writer.name, err)
			}
		})
		if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, $1)`, schedule.WindowLockKey); err != nil {
			t.Fatal(err)
		}

		finished := make(chan error, 1)
		go func() { finished <- writer.run(store) }()
		if !waitingOnTheAdvisoryLock(t, pool, application) {
			t.Errorf("%s never waited for the maintenance-windows lock that window-schedule holds exclusively", writer.name)
		}
		early := false
		select {
		case err := <-finished:
			t.Errorf("%s finished (%v) while the maintenance-windows lock was held exclusively", writer.name, err)
			early = true
		default:
		}
		if err := holder.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if early {
			continue
		}
		select {
		case err := <-finished:
			if err != nil {
				t.Errorf("%s after the lock was released: %v", writer.name, err)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("%s never finished after the lock was released", writer.name)
		}
	}
}
