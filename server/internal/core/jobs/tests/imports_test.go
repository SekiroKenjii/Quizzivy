package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
)

type fakeRunner struct {
	mu      sync.Mutex
	polls   []time.Time
	run     func(call int) (bool, error)
	nextDue func() (time.Time, bool, error)
}

func (f *fakeRunner) RunOne(context.Context) (bool, error) {
	f.mu.Lock()
	f.polls = append(f.polls, time.Now())
	call := len(f.polls)
	f.mu.Unlock()
	if f.run == nil {
		return false, nil
	}
	return f.run(call)
}

func (f *fakeRunner) NextDue(context.Context) (time.Time, bool, error) {
	if f.nextDue == nil {
		return time.Time{}, false, nil
	}
	return f.nextDue()
}

func (f *fakeRunner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.polls)
}

func runInBackground(t *testing.T, runner *fakeRunner, schedule jobs.ImportSchedule) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- jobs.RunImports(ctx, slog.Default(), runner, schedule) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("RunImports: %v", err)
		}
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestImportPollDrainsSeriallyWithoutWaitingBetweenCompletedRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runner := &fakeRunner{}
	runner.run = func(call int) (bool, error) {
		if call == 3 {
			cancel()
		}
		return true, nil
	}
	err := jobs.RunImports(ctx, slog.Default(), runner, jobs.ImportSchedule{Idle: time.Hour, Retry: time.Hour})
	if err != nil || runner.count() != 3 {
		t.Fatalf("serial drain: calls=%d err=%v", runner.count(), err)
	}
}

func TestImportPollStopsPromptlyAndDoesNotLogRawFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var logs bytes.Buffer
	runner := &fakeRunner{run: func(int) (bool, error) {
		cancel()
		return false, errors.New("private source content and storage credentials")
	}}
	err := jobs.RunImports(ctx, slog.New(slog.NewJSONHandler(&logs, nil)), runner, jobs.ImportSchedule{Idle: time.Hour, Retry: time.Hour})
	if err != nil || runner.count() != 1 || strings.Contains(logs.String(), "credentials") || !strings.Contains(logs.String(), "WORKER_POLL_FAILED") {
		t.Fatalf("unsafe polling: calls=%d err=%v logs=%s", runner.count(), err, logs.String())
	}
}

func TestImportPollRejectsABusyLoopSchedule(t *testing.T) {
	for _, schedule := range []jobs.ImportSchedule{{Idle: time.Hour}, {Idle: time.Second, Retry: time.Minute}} {
		runner := &fakeRunner{}
		if err := jobs.RunImports(context.Background(), slog.Default(), runner, schedule); err == nil || runner.count() != 0 {
			t.Fatalf("accepted %+v", schedule)
		}
	}
}

func TestAnIdleWorkerWaitsForAWakeInsteadOfPolling(t *testing.T) {
	wake := make(chan struct{}, 1)
	runner := &fakeRunner{}
	runInBackground(t, runner, jobs.ImportSchedule{Wake: wake, Idle: time.Hour, Retry: time.Millisecond})

	waitFor(t, "the first poll", func() bool { return runner.count() == 1 })
	time.Sleep(100 * time.Millisecond)
	if runner.count() != 1 {
		t.Fatalf("an idle worker polled %d times without being woken", runner.count())
	}
	wake <- struct{}{}
	waitFor(t, "the poll after a wake", func() bool { return runner.count() == 2 })
}

func TestAnIdleWorkerPollsWhenQueuedWorkFallsDue(t *testing.T) {
	due := time.Now().Add(60 * time.Millisecond)
	runner := &fakeRunner{nextDue: func() (time.Time, bool, error) { return due, true, nil }}
	runInBackground(t, runner, jobs.ImportSchedule{Idle: time.Hour, Retry: time.Millisecond})

	waitFor(t, "the poll when work falls due", func() bool { return runner.count() >= 2 })
	runner.mu.Lock()
	second := runner.polls[1]
	runner.mu.Unlock()
	if second.Before(due) {
		t.Fatalf("polled %v before the work fell due", due.Sub(second))
	}
}

func TestWorkAlreadyDueStillWaitsTheRetryPause(t *testing.T) {
	runner := &fakeRunner{nextDue: func() (time.Time, bool, error) { return time.Now().Add(-time.Hour), true, nil }}
	runInBackground(t, runner, jobs.ImportSchedule{Idle: time.Hour, Retry: 40 * time.Millisecond})

	waitFor(t, "three polls", func() bool { return runner.count() >= 3 })
	runner.mu.Lock()
	defer runner.mu.Unlock()
	for i := 1; i < len(runner.polls); i++ {
		if gap := runner.polls[i].Sub(runner.polls[i-1]); gap < 40*time.Millisecond {
			t.Fatalf("polls %d and %d were %v apart, under the retry pause", i-1, i, gap)
		}
	}
}

func TestAWakeIsAcknowledgedAndCoalesced(t *testing.T) {
	wake := make(chan struct{}, 1)
	handler := jobs.WakeHandler(wake)
	for range 3 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/wake", nil))
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
			t.Fatalf("wake answered %d %q", rec.Code, rec.Body.String())
		}
	}
	if len(wake) != 1 {
		t.Fatalf("pending wakes = %d, want them coalesced into 1", len(wake))
	}
	for _, req := range []*http.Request{httptest.NewRequest(http.MethodGet, "/wake", nil), httptest.NewRequest(http.MethodPost, "/", nil)} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code < 400 {
			t.Fatalf("%s %s answered %d", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestTheWakeListenerRefusesAnUnusableAddress(t *testing.T) {
	if err := jobs.ServeWake(context.Background(), slog.Default(), "not-an-address", make(chan struct{}, 1)); err == nil {
		t.Fatal("a bad wake address was accepted")
	}
}

func TestAFloodOfWakesStillPollsNoFasterThanTheRetryPause(t *testing.T) {
	wake := make(chan struct{}, 1)
	runner := &fakeRunner{}
	runInBackground(t, runner, jobs.ImportSchedule{Wake: wake, Idle: time.Hour, Retry: 40 * time.Millisecond})
	stop := time.After(300 * time.Millisecond)
	for flooding := true; flooding; {
		select {
		case <-stop:
			flooding = false
		case wake <- struct{}{}:
		default:
			time.Sleep(time.Millisecond)
		}
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if len(runner.polls) < 3 {
		t.Fatalf("wakes produced only %d polls", len(runner.polls))
	}
	for i := 1; i < len(runner.polls); i++ {
		if gap := runner.polls[i].Sub(runner.polls[i-1]); gap < 40*time.Millisecond {
			t.Fatalf("polls %d and %d were %v apart under a flood of wakes", i-1, i, gap)
		}
	}
}
