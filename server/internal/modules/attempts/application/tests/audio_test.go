//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecordingAPlayCountsFromOne(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	first, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice})
	if err != nil {
		t.Fatalf("first play: %v", err)
	}
	if first.Plays != 1 {
		t.Errorf("plays %d, want 1", first.Plays)
	}
	second, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice})
	if err != nil {
		t.Fatalf("second play: %v", err)
	}
	if second.Plays != 2 {
		t.Errorf("plays %d, want 2", second.Plays)
	}
}

// The reason this is one statement. A read-then-write loses every increment
// that lands between the two, and a student tapping replay on a flaky
// connection is exactly the case that produces overlapping requests.
func TestTenConcurrentPlaysCountTen(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	const taps = 10
	var wg sync.WaitGroup
	errs := make([]error, taps)
	start := make(chan struct{})
	for i := range taps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = svc.Commands.RecordPlay.Handle(context.Background(), command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice})
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("tap %d: %v", i, err)
		}
	}
	got := count(t, pool, `
		SELECT plays FROM app.attempt_audio_plays
		 WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		session.Attempt.ID, w.choice)
	if got != taps {
		t.Errorf("plays = %d after %d concurrent taps, want %d", got, taps, taps)
	}
}

// [§11.4] Over-limit is reported, never enforced. Blocking would punish bad
// wifi far more often than it would catch anyone.
func TestAPlayBeyondTheLimitSucceedsAndReportsTheHigherCount(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.test_version_questions SET audio_max_plays = 2 WHERE id = $1::uuid`,
		w.listening); err != nil {
		t.Fatal(err)
	}

	var last domain.Plays
	for i := range 4 {
		got, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.listening})
		if err != nil {
			t.Fatalf("play %d was refused: %v", i+1, err)
		}
		last = got
	}
	if last.Plays != 4 {
		t.Errorf("plays %d, want 4", last.Plays)
	}
	if last.MaxPlays == nil || *last.MaxPlays != 2 {
		t.Errorf("maxPlays %v, want 2 — the teacher needs the limit to compare against", last.MaxPlays)
	}
}

func TestPlaysOnSomeoneElsesAttemptAreRefused(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	if _, err := svc.Commands.RecordPlay.Handle(context.Background(), command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.outsider, QuestionID: w.choice}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

func TestAPlayOnAQuestionOutsideThePaperIsRefused(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	other := seedWorld(t, pool, openAssignment())
	ctx := context.Background()

	for _, c := range []struct {
		name     string
		question string
	}{
		{"another paper's question", other.choice},
		{"a question that does not exist", uuid.NewString()},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: c.question}); !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("got %v, want ErrForbidden", err)
			}
		})
	}
}

// The count survives the reload that destroyed whatever the client was
// counting, which is the whole reason it lives on the server.
func TestTheReloadedPayloadCarriesThePlaysAlreadyRecorded(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	for range 3 {
		if _, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice}); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: session.Attempt.ID, StudentID: w.student})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := reloaded.AudioPlays[w.choice]; got != 3 {
		t.Errorf("audioPlays[%s] = %d, want 3", w.choice, got)
	}
}

func playCount(t *testing.T, pool *pgxpool.Pool, attemptID string) int {
	t.Helper()
	return count(t, pool, `
		SELECT COALESCE(sum(plays), 0)::int FROM app.attempt_audio_plays
		 WHERE attempt_id = $1::uuid`, attemptID)
}

func TestAPlayFromTheCurrentSessionCounts(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	got, err := svc.Commands.RecordPlay.Handle(context.Background(), command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice, SessionID: session.SessionID})
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	if got.Plays != 1 {
		t.Errorf("plays %d, want 1", got.Plays)
	}
}

func TestAPlayFromASupersededSessionIsRefusedAndCountsNothing(t *testing.T) {
	pool := newPool(t)
	svc, w, first := started(t, pool)
	ctx := context.Background()

	if _, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if _, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: first.Attempt.ID, StudentID: w.student, QuestionID: w.choice, SessionID: first.SessionID}); !errors.Is(err, domain.ErrSessionSuperseded) {
		t.Fatalf("got %v, want ErrSessionSuperseded", err)
	}
	if n := playCount(t, pool, first.Attempt.ID); n != 0 {
		t.Errorf("%d plays counted for a superseded session, want 0", n)
	}
}

func TestAPlayWithNoSessionStillCountsDuringTheExpandRelease(t *testing.T) {
	pool := newPool(t)
	svc, w, first := started(t, pool)
	ctx := context.Background()

	if _, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if _, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: first.Attempt.ID, StudentID: w.student, QuestionID: w.choice}); err != nil {
		t.Fatalf("play with no session: %v", err)
	}
	if n := playCount(t, pool, first.Attempt.ID); n != 1 {
		t.Errorf("%d plays, want 1", n)
	}
}

func TestAPlayAfterTheDeadlineIsRefusedAndCountsNothing(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts
		    SET started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		  WHERE id = $1::uuid`, session.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice, SessionID: session.SessionID}); !errors.Is(err, domain.ErrDeadlinePassed) {
		t.Fatalf("got %v, want ErrDeadlinePassed", err)
	}
	if n := playCount(t, pool, session.Attempt.ID); n != 0 {
		t.Errorf("%d plays counted after the deadline, want 0", n)
	}
}

func TestAPlayOnASubmittedAttemptIsRefusedAndCountsNothing(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET status='submitted', submitted_at=now() WHERE id=$1::uuid`,
		session.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: session.Attempt.ID, StudentID: w.student, QuestionID: w.choice}); !errors.Is(err, domain.ErrAttemptClosed) {
		t.Fatalf("got %v, want ErrAttemptClosed", err)
	}
	if n := playCount(t, pool, session.Attempt.ID); n != 0 {
		t.Errorf("%d plays counted on a submitted attempt, want 0", n)
	}
}
