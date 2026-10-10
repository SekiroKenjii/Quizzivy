//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	assignmentsdomain "quizzivy/internal/modules/assignments/domain"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pausedGuard struct {
	inner   repositories.StartGuard
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

func newPausedGuard() *pausedGuard {
	return &pausedGuard{inner: adapters.AttemptStartGuard{}, reached: make(chan struct{}), release: make(chan struct{})}
}

func (g *pausedGuard) GuardAttemptStart(ctx context.Context, tx pgx.Tx, now, deadline time.Time) (*domain.MaintenanceWindow, error) {
	window, err := g.inner.GuardAttemptStart(ctx, tx, now, deadline)
	g.once.Do(func() {
		close(g.reached)
		<-g.release
	})
	return window, err
}

func heldDownByTheClose() worldOpts {
	o := openAssignment()
	o.closesAt = time.Now().Add(30 * time.Minute)
	return o
}

func extend(pool *pgxpool.Pool, w world, minutes int) error {
	req := assignmentsdomain.Request{ID: w.assignment, ActorID: w.admin, All: true, IP: "203.0.113.7", UserAgent: "go-test"}
	_, err := assignmentsrepo.NewPostgres(db.NewContext(pool)).Extend(context.Background(), req, minutes, false, time.Now())
	return err
}

func deadlineOf(t *testing.T, pool *pgxpool.Pool, attempt string) time.Time {
	t.Helper()
	var deadline time.Time
	if err := pool.QueryRow(context.Background(), `SELECT deadline_at FROM app.attempts WHERE id = $1::uuid`, attempt).Scan(&deadline); err != nil {
		t.Fatal(err)
	}
	return deadline
}

func TestAStartThatReadItsRulesBeforeAnExtensionStoresTheExtendedDeadline(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, heldDownByTheClose())
	guard := newPausedGuard()
	svc := application.New(nil, nil, repositories.NewPostgres(db.NewContext(pool), guard))

	var session domain.Session
	var startErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		session, startErr = svc.Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	}()
	select {
	case <-guard.reached:
	case <-done:
		t.Fatalf("the start ended before it reached the maintenance check: %v", startErr)
	case <-time.After(30 * time.Second):
		t.Fatal("the start never reached the maintenance check")
	}

	if err := extend(pool, w, 120); err != nil {
		t.Fatalf("extend while the start waits: %v", err)
	}
	close(guard.release)
	<-done
	if startErr != nil {
		t.Fatalf("start: %v", startErr)
	}

	want := session.Attempt.StartedAt.Add(60 * time.Minute)
	if got := deadlineOf(t, pool, session.Attempt.ID); !got.Equal(want) {
		t.Errorf("an attempt that began after the extension committed has deadline %v, want its start plus 60 minutes, %v: it was cut off by the close it read before", got, want)
	}
	if !session.Attempt.DeadlineAt.Equal(want) {
		t.Errorf("the session says the deadline is %v, want %v", session.Attempt.DeadlineAt, want)
	}
}

func TestAnExtensionThatWaitsForAStartRecomputesTheAttemptItInserted(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, heldDownByTheClose())
	ctx := context.Background()

	starting, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = starting.Rollback(ctx) })
	var closes time.Time
	if err := starting.QueryRow(ctx, `SELECT closes_at FROM app.assignments WHERE id = $1::uuid FOR SHARE`, w.assignment).Scan(&closes); err != nil {
		t.Fatal(err)
	}
	var attempt string
	if err := starting.QueryRow(ctx, `
		INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, gen_random_uuid(), 1, sha256('b'::bytea), now(), $4)
		RETURNING id::text`, w.assignment, w.versionID, w.student, closes).Scan(&attempt); err != nil {
		t.Fatal(err)
	}

	finished := make(chan error, 1)
	go func() { finished <- extend(pool, w, 120) }()
	waitUntilBlocked(t, pool, "assignment.extended")
	select {
	case err := <-finished:
		t.Fatalf("the extension finished (%v) while a start held the assignment row", err)
	default:
	}

	if err := starting.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("extend: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the extension never finished after the start committed")
	}

	var started time.Time
	if err := pool.QueryRow(ctx, `SELECT started_at FROM app.attempts WHERE id = $1::uuid`, attempt).Scan(&started); err != nil {
		t.Fatal(err)
	}
	if got, want := deadlineOf(t, pool, attempt), started.Add(60*time.Minute); !got.Equal(want) {
		t.Errorf("the attempt inserted before the extension committed has deadline %v, want %v", got, want)
	}
}

func waitUntilBlocked(t *testing.T, pool *pgxpool.Pool, marker string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			 WHERE wait_event_type = 'Lock' AND query LIKE '%' || $1 || '%' AND pid <> pg_backend_pid()`, marker).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the statement never waited on a lock")
}

func TestStartingAndExtendingAtTheSameMomentLeaveTheCommittedWindowOnTheAttempt(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, heldDownByTheClose())
	people := newCohort(t, pool, w)
	svc := newService(t, pool)
	ctx := context.Background()

	for round := range 16 {
		student := people.student(t)
		if _, err := pool.Exec(ctx, `UPDATE app.assignments SET closes_at = now() + interval '30 minutes' WHERE id = $1::uuid`, w.assignment); err != nil {
			t.Fatal(err)
		}
		var session domain.Session
		var startErr, extendErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			session, startErr = svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: student})
		}()
		go func() {
			defer wg.Done()
			extendErr = extend(pool, w, 120)
		}()
		wg.Wait()
		if startErr != nil || extendErr != nil {
			t.Fatalf("round %d: start %v, extend %v", round, startErr, extendErr)
		}
		want := session.Attempt.StartedAt.Add(60 * time.Minute)
		if got := deadlineOf(t, pool, session.Attempt.ID); !got.Equal(want) {
			t.Fatalf("round %d: whichever committed first, the attempt has deadline %v, want %v", round, got, want)
		}
	}
}

func TestExtendingResumingAndSubmittingOneAttemptAtOnceNeitherDeadlockNorShorten(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, heldDownByTheClose())
	people := newCohort(t, pool, w)
	svc := newService(t, pool)
	ctx := context.Background()

	for round := range 16 {
		student := people.student(t)
		if _, err := pool.Exec(ctx, `UPDATE app.assignments SET closes_at = now() + interval '30 minutes' WHERE id = $1::uuid`, w.assignment); err != nil {
			t.Fatal(err)
		}
		live, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: student})
		if err != nil {
			t.Fatalf("round %d: start: %v", round, err)
		}
		before := live.Attempt.DeadlineAt

		var extendErr, resumeErr, submitErr error
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); extendErr = extend(pool, w, 120) }()
		go func() {
			defer wg.Done()
			_, resumeErr = svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: student, Resume: live.Attempt.ID})
		}()
		go func() {
			defer wg.Done()
			_, submitErr = svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: live.Attempt.ID, StudentID: student, Reason: domain.Manual})
		}()
		wg.Wait()

		if extendErr != nil || submitErr != nil {
			t.Fatalf("round %d: extend %v, submit %v", round, extendErr, submitErr)
		}
		if resumeErr != nil && !errors.Is(resumeErr, domain.ErrAttemptClosed) && !errors.Is(resumeErr, domain.ErrNotFound) {
			t.Fatalf("round %d: resume: %v", round, resumeErr)
		}
		got, lengthened := deadlineOf(t, pool, live.Attempt.ID), live.Attempt.StartedAt.Add(60*time.Minute)
		if !got.Equal(before) && !got.Equal(lengthened) {
			t.Fatalf("round %d: the deadline is %v, want the one it had, %v, or the rule's, %v", round, got, before, lengthened)
		}
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM app.attempts WHERE id = $1::uuid`, live.Attempt.ID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != string(domain.Submitted) {
			t.Fatalf("round %d: the attempt is %s, want submitted", round, status)
		}
	}
}
