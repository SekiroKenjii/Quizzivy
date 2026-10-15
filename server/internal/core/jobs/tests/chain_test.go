package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
)

func TestTheFirstQueueWithWorkIsServedAndTheRestWait(t *testing.T) {
	first := &fakeRunner{run: func(int) (bool, error) { return true, nil }}
	second := &fakeRunner{run: func(int) (bool, error) { return true, nil }}
	worked, err := jobs.ImportRunners{first, second}.RunOne(context.Background())
	if err != nil || !worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
	if len(first.polls) != 1 || len(second.polls) != 0 {
		t.Errorf("polled the first %d times and the second %d, want 1 and 0", len(first.polls), len(second.polls))
	}
}

func TestAnIdleQueueLetsTheNextOneRun(t *testing.T) {
	idle := &fakeRunner{}
	busy := &fakeRunner{run: func(int) (bool, error) { return true, nil }}
	worked, err := jobs.ImportRunners{idle, busy}.RunOne(context.Background())
	if err != nil || !worked || len(idle.polls) != 1 || len(busy.polls) != 1 {
		t.Fatalf("RunOne = %v, %v after %d and %d polls", worked, err, len(idle.polls), len(busy.polls))
	}
	if worked, err := (jobs.ImportRunners{&fakeRunner{}, &fakeRunner{}}).RunOne(context.Background()); err != nil || worked {
		t.Errorf("two idle queues reported work: %v, %v", worked, err)
	}
}

func TestAnErrorStopsTheChain(t *testing.T) {
	broken := errors.New("database down")
	first := &fakeRunner{run: func(int) (bool, error) { return false, broken }}
	second := &fakeRunner{run: func(int) (bool, error) { return true, nil }}
	worked, err := jobs.ImportRunners{first, second}.RunOne(context.Background())
	if !errors.Is(err, broken) || worked || len(second.polls) != 0 {
		t.Fatalf("RunOne = %v, %v; the second queue polled %d times", worked, err, len(second.polls))
	}
}

func TestTheChainFallsDueWhenItsEarliestQueueDoes(t *testing.T) {
	now := time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC)
	due := func(at time.Time, ok bool) *fakeRunner {
		return &fakeRunner{nextDue: func() (time.Time, bool, error) { return at, ok, nil }}
	}
	got, ok, err := jobs.ImportRunners{due(now.Add(time.Hour), true), due(now.Add(time.Minute), true), due(now, false)}.NextDue(context.Background())
	if err != nil || !ok || !got.Equal(now.Add(time.Minute)) {
		t.Fatalf("NextDue = %v, %v, %v", got, ok, err)
	}
	if _, ok, err := (jobs.ImportRunners{due(now, false), due(now, false)}).NextDue(context.Background()); err != nil || ok {
		t.Errorf("queues with nothing waiting reported a due time: %v, %v", ok, err)
	}
	if _, ok, err := (jobs.ImportRunners{}).NextDue(context.Background()); err != nil || ok {
		t.Errorf("an empty chain reported a due time: %v, %v", ok, err)
	}
}

func TestADueErrorFromAnyQueueIsReported(t *testing.T) {
	broken := errors.New("database down")
	failing := &fakeRunner{nextDue: func() (time.Time, bool, error) { return time.Time{}, false, broken }}
	if _, _, err := (jobs.ImportRunners{&fakeRunner{}, failing}).NextDue(context.Background()); !errors.Is(err, broken) {
		t.Fatalf("error = %v", err)
	}
}
