package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
	"quizzivy/internal/platform/email"
)

type scriptedDispatcher struct {
	mu       sync.Mutex
	batches  []email.Batch
	errs     []error
	dues     []time.Time
	calls    atomic.Int32
	nextDues atomic.Int32
	called   chan struct{}
}

func (d *scriptedDispatcher) Dispatch(context.Context) (email.Batch, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := int(d.calls.Add(1)) - 1
	defer func() {
		select {
		case d.called <- struct{}{}:
		default:
		}
	}()
	var batch email.Batch
	if n < len(d.batches) {
		batch = d.batches[n]
	}
	var err error
	if n < len(d.errs) {
		err = d.errs[n]
	}
	return batch, err
}

func (d *scriptedDispatcher) NextDue(context.Context) (time.Time, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := int(d.nextDues.Add(1)) - 1
	if n < len(d.dues) {
		return d.dues[n], !d.dues[n].IsZero(), nil
	}
	return time.Time{}, false, nil
}

func runEmail(t *testing.T, d *scriptedDispatcher, s jobs.EmailSchedule) (stop func(), logs *bytes.Buffer) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	logs = &bytes.Buffer{}
	done := make(chan struct{})
	var mu sync.Mutex
	logger := slog.New(slog.NewJSONHandler(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return logs.Write(p)
	}), nil))
	go func() {
		jobs.RunEmail(ctx, logger, d, s)
		close(done)
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("RunEmail did not stop with its context")
		}
	}, logs
}

func waitForCalls(t *testing.T, d *scriptedDispatcher, n int32) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for d.calls.Load() < n {
		select {
		case <-d.called:
		case <-deadline:
			t.Fatalf("only %d dispatches ran, want %d", d.calls.Load(), n)
		}
	}
}

func TestAnEmptyOutboxIsDispatchedOnceAtStartAndThenLeftAloneUntilWoken(t *testing.T) {
	d := &scriptedDispatcher{called: make(chan struct{}, 8)}
	wake := make(chan struct{}, 1)
	stop, _ := runEmail(t, d, jobs.EmailSchedule{Wake: wake, Retry: time.Hour, Busy: time.Hour, Floor: time.Millisecond})
	defer stop()
	waitForCalls(t, d, 1)
	time.Sleep(150 * time.Millisecond)
	if got := d.calls.Load(); got != 1 {
		t.Fatalf("an idle outbox was dispatched %d times; it must not wake the database on a timer", got)
	}
	wake <- struct{}{}
	waitForCalls(t, d, 2)
}

func TestARowFallingDueArmsATimerAndAFullBatchIsFollowedAtOnce(t *testing.T) {
	d := &scriptedDispatcher{
		called:  make(chan struct{}, 8),
		batches: []email.Batch{{Claimed: 10, Sent: 10}, {}, {}},
		dues:    []time.Time{time.Now().Add(80 * time.Millisecond)},
	}
	stop, _ := runEmail(t, d, jobs.EmailSchedule{Wake: make(chan struct{}), Retry: time.Hour, Busy: time.Hour, Floor: time.Millisecond})
	defer stop()
	waitForCalls(t, d, 3)
	if d.nextDues.Load() < 1 {
		t.Fatal("the schedule never asked when the next row falls due")
	}
}

func TestAFailedDispatchIsLoggedWithoutDetailAndRetriedAfterTheRetryPause(t *testing.T) {
	d := &scriptedDispatcher{called: make(chan struct{}, 8), errs: []error{errors.New("password=hunter2 refused")}}
	stop, logs := runEmail(t, d, jobs.EmailSchedule{Wake: make(chan struct{}), Retry: 40 * time.Millisecond, Busy: time.Hour, Floor: time.Millisecond})
	defer stop()
	waitForCalls(t, d, 2)
	out := logs.String()
	if !bytes.Contains([]byte(out), []byte(`"code":"EMAIL_DISPATCH_FAILED"`)) || bytes.Contains([]byte(out), []byte("hunter2")) {
		t.Fatalf("log: %s", out)
	}
}

func TestABusyDispatchWaitsTheBusyPauseNotTheRetryPause(t *testing.T) {
	d := &scriptedDispatcher{called: make(chan struct{}, 8), batches: []email.Batch{{Busy: true}}}
	stop, _ := runEmail(t, d, jobs.EmailSchedule{Wake: make(chan struct{}), Retry: time.Hour, Busy: 30 * time.Millisecond, Floor: time.Millisecond})
	defer stop()
	waitForCalls(t, d, 2)
}
