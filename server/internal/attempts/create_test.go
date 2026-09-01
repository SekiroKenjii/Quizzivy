package attempts_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/attempts"
)

func TestStartingAnAttemptDealsThePaperAndOpensASession(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)

	got, err := svc.StartOrResume(context.Background(), w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	switch {
	case got.Attempt.AttemptNo != 1:
		t.Errorf("attemptNo %d, want 1", got.Attempt.AttemptNo)
	case got.Attempt.Status != attempts.InProgress:
		t.Errorf("status %q, want in_progress", got.Attempt.Status)
	case got.SessionID == "":
		t.Error("no session id")
	case got.BeaconToken == "":
		t.Error("no beacon token")
	case len(got.Questions) != 4:
		t.Errorf("%d questions, want 4", len(got.Questions))
	}

	if d := got.Attempt.DeadlineAt.Sub(got.Attempt.StartedAt); d < 59*time.Minute || d > 61*time.Minute {
		t.Errorf("deadline is %v after the start, want about 60m", d)
	}
}

// [D-03] The token is a bearer credential. Storing it as written would make the
// database a source of live event-write access to every attempt at once.
func TestTheBeaconTokenIsStoredHashedAndNeverInTheClear(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	got, err := newService(t, pool).StartOrResume(context.Background(), w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var matches int
	err = pool.QueryRow(context.Background(), `
		SELECT count(*) FROM app.attempts
		 WHERE id = $1::uuid AND beacon_token_hash = sha256($2::bytea)`,
		got.Attempt.ID, []byte(got.BeaconToken)).Scan(&matches)
	if err != nil {
		t.Fatal(err)
	}
	if matches != 1 {
		t.Fatal("the stored hash is not sha256 of the issued token")
	}

	var clear int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM app.attempts
		 WHERE id = $1::uuid AND beacon_token_hash = $2::bytea`,
		got.Attempt.ID, []byte(got.BeaconToken)).Scan(&clear); err != nil {
		t.Fatal(err)
	}
	if clear != 0 {
		t.Fatal("the token was stored in the clear")
	}
}

// The double tap. A student on a slow connection taps "Bắt đầu" twice and must
// end up in one attempt, not two -- and certainly not with a 500.
func TestTwoConcurrentStartsYieldOneAttempt(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)

	const racers = 8
	var wg sync.WaitGroup
	sessions := make([]attempts.Session, racers)
	errs := make([]error, racers)
	start := make(chan struct{})

	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			sessions[i], errs[i] = svc.StartOrResume(context.Background(), w.assignment, w.student)
		}()
	}
	close(start)
	wg.Wait()

	ids := map[string]bool{}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d failed: %v", i, err)
		}
		ids[sessions[i].Attempt.ID] = true
	}
	if len(ids) != 1 {
		t.Errorf("%d distinct attempts came back, want 1: %v", len(ids), ids)
	}

	var rows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.attempts WHERE assignment_id = $1::uuid AND student_id = $2::uuid`,
		w.assignment, w.student).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("%d attempt rows exist, want 1", rows)
	}
}

func TestAStudentNotTargetedByTheAssignmentCannotStartIt(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	_, err := newService(t, pool).StartOrResume(context.Background(), w.assignment, w.outsider)
	if !errors.Is(err, attempts.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestAnAssignmentOutsideItsWindowCannotBeStarted(t *testing.T) {
	now := time.Now()
	closed := now.Add(-time.Minute)

	cases := []struct {
		name string
		opts worldOpts
		want error
	}{
		{"not open yet", worldOpts{
			opensAt: now.Add(time.Hour), closesAt: now.Add(2 * time.Hour),
			maxAttempts: 1, duration: 60,
		}, attempts.ErrAssignmentClosed},
		{"already past its close", worldOpts{
			opensAt: now.Add(-3 * time.Hour), closesAt: now.Add(-time.Hour),
			maxAttempts: 1, duration: 60,
		}, attempts.ErrAssignmentClosed},
		{"closed early by the teacher", worldOpts{
			opensAt: now.Add(-time.Hour), closesAt: now.Add(time.Hour), closedAt: &closed,
			maxAttempts: 1, duration: 60,
		}, attempts.ErrAssignmentClosed},
		// A draft is not "not yet" -- there is nothing to wait for, and saying
		// so would tell a student an unpublished assignment exists.
		{"still a draft", worldOpts{
			opensAt: now.Add(-time.Hour), closesAt: now.Add(time.Hour), draft: true,
			maxAttempts: 1, duration: 60,
		}, attempts.ErrNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pool := newPool(t)
			w := seedWorld(t, pool, c.opts)
			_, err := newService(t, pool).StartOrResume(context.Background(), w.assignment, w.student)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestAStudentCannotStartMoreAttemptsThanTheyWereGiven(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET status='submitted', submitted_at=now() WHERE id=$1::uuid`,
		first.Attempt.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.StartOrResume(ctx, w.assignment, w.student); !errors.Is(err, attempts.ErrLimitReached) {
		t.Fatalf("got %v, want ErrLimitReached", err)
	}
}

// Voiding is how a teacher gives an attempt back. One that still counted
// against the limit would not be given back at all.
func TestAVoidedAttemptDoesNotSpendTheStudentsLastTry(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	first, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET status='voided', void_reason='mất điện' WHERE id=$1::uuid`,
		first.Attempt.ID); err != nil {
		t.Fatal(err)
	}

	second, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("start after void: %v", err)
	}
	if second.Attempt.ID == first.Attempt.ID {
		t.Fatal("resumed the voided attempt instead of starting a new one")
	}
	if second.Attempt.AttemptNo != 2 {
		t.Errorf("attemptNo %d, want 2", second.Attempt.AttemptNo)
	}
}
