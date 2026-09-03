package attempts_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/attempts"
)

func TestRecordingAPlayCountsFromOne(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	first, err := svc.RecordPlay(ctx, session.Attempt.ID, w.student, w.choice)
	if err != nil {
		t.Fatalf("first play: %v", err)
	}
	if first.Plays != 1 {
		t.Errorf("plays %d, want 1", first.Plays)
	}
	second, err := svc.RecordPlay(ctx, session.Attempt.ID, w.student, w.choice)
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
			_, errs[i] = svc.RecordPlay(context.Background(), session.Attempt.ID, w.student, w.choice)
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

	var last attempts.Plays
	for i := range 4 {
		got, err := svc.RecordPlay(ctx, session.Attempt.ID, w.student, w.listening)
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

	if _, err := svc.RecordPlay(context.Background(), session.Attempt.ID, w.outsider, w.choice); !errors.Is(err, attempts.ErrForbidden) {
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
			if _, err := svc.RecordPlay(ctx, session.Attempt.ID, w.student, c.question); !errors.Is(err, attempts.ErrForbidden) {
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
		if _, err := svc.RecordPlay(ctx, session.Attempt.ID, w.student, w.choice); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := svc.Get(ctx, session.Attempt.ID, w.student)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := reloaded.AudioPlays[w.choice]; got != 3 {
		t.Errorf("audioPlays[%s] = %d, want 3", w.choice, got)
	}
}
