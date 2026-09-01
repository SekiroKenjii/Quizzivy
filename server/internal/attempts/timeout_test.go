package attempts_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

// expire moves the whole attempt into the past, keeping
// CHECK (deadline_at > started_at) satisfied.
func expire(t *testing.T, pool *pgxpool.Pool, attemptID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		UPDATE app.attempts
		   SET started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		 WHERE id = $1::uuid`, attemptID); err != nil {
		t.Fatal(err)
	}
}

// A deadline takes effect with nothing watching the clock. There is no
// scheduler: the read that had to happen anyway is what closes the attempt.
func TestAnAttemptPastItsDeadlineReadsBackAsTimedOut(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()

	expire(t, pool, session.Attempt.ID)

	reloaded, err := svc.Get(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if reloaded.Attempt.Status != attempts.TimedOut {
		t.Errorf("status %q, want timed_out", reloaded.Attempt.Status)
	}
}

// Graded the same way. Running out of time is not a reason to lose the marks
// already earned.
func TestATimedOutAttemptIsStillGraded(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()

	expire(t, pool, session.Attempt.ID)
	if _, err := svc.Get(ctx, session.Attempt.ID, w.student); err != nil {
		t.Fatalf("get: %v", err)
	}

	var earned, total float64
	if err := pool.QueryRow(ctx,
		`SELECT score_earned, score_total FROM app.attempts WHERE id = $1::uuid`,
		session.Attempt.ID).Scan(&earned, &total); err != nil {
		t.Fatal(err)
	}
	if earned != 15 {
		t.Errorf("earned %v, want 15 — the objective answers were already given", earned)
	}
	if total != 10 {
		t.Errorf("total %v, want 10", total)
	}
}

// It ended when the clock did, not when somebody happened to look.
func TestTheEndTimeIsTheDeadlineNotTheMomentItWasNoticed(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	expire(t, pool, session.Attempt.ID)
	if _, err := svc.Get(ctx, session.Attempt.ID, w.student); err != nil {
		t.Fatal(err)
	}

	var matches bool
	if err := pool.QueryRow(ctx,
		`SELECT submitted_at = deadline_at FROM app.attempts WHERE id = $1::uuid`,
		session.Attempt.ID).Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Error("submitted_at is not the deadline; a timeout ended when the clock did")
	}
}

// [00025] The teacher's queue has to see it. An essay that ran out of time is
// still an essay somebody has to read, and this is the count the dashboard
// renders.
func TestATimedOutEssayStillReachesTheGradingQueue(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()

	expire(t, pool, session.Attempt.ID)
	if _, err := svc.Get(ctx, session.Attempt.ID, w.student); err != nil {
		t.Fatal(err)
	}

	queued := count(t, pool, `
		SELECT count(*) FROM app.attempt_answers ans
		  JOIN app.attempts at ON at.id = ans.attempt_id
		 WHERE at.id = $1::uuid
		   AND ans.requires_manual AND ans.manual_score IS NULL
		   AND at.status IN ('submitted','timed_out')`, session.Attempt.ID)
	if queued != 1 {
		t.Errorf("%d answers awaiting a teacher, want 1", queued)
	}
}

// Resuming must not hand back a paper whose time is already gone. Closing it
// here is what lets a student with an attempt left start a fresh one.
func TestResumingAnExpiredAttemptClosesItRatherThanReopeningIt(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	expire(t, pool, session.Attempt.ID)

	// max_attempts is 1 in the fixture, so the student has none left and the
	// refusal proves the old one was closed rather than resumed.
	_, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err == nil {
		t.Fatal("StartOrResume handed back an attempt whose deadline had passed")
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status::text FROM app.attempts WHERE id = $1::uuid`,
		session.Attempt.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(attempts.TimedOut) {
		t.Errorf("status %q, want timed_out", status)
	}
}

// Expiry is idempotent: two readers arriving at once must not grade twice.
func TestExpiringTwiceIsHarmless(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()

	expire(t, pool, session.Attempt.ID)
	for range 3 {
		if _, err := svc.Get(ctx, session.Attempt.ID, w.student); err != nil {
			t.Fatalf("get: %v", err)
		}
	}
	var earned float64
	if err := pool.QueryRow(ctx,
		`SELECT score_earned FROM app.attempts WHERE id = $1::uuid`,
		session.Attempt.ID).Scan(&earned); err != nil {
		t.Fatal(err)
	}
	if earned != 15 {
		t.Errorf("earned %v after three reads, want 15", earned)
	}
}
