package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"quizzivy/internal/core/jobs"

	identityapp "quizzivy/internal/modules/identity/application"
	identitycommand "quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/shared/cqrs"
)

type housekeepingLog struct {
	mu    sync.Mutex
	calls []string
	out   bytes.Buffer
}

func (l *housekeepingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.Write(p)
}

func (l *housekeepingLog) logger() *slog.Logger { return slog.New(slog.NewJSONHandler(l, nil)) }

func (l *housekeepingLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.String()
}

func (l *housekeepingLog) step(name string, batches ...func(limit int) (int, error)) jobs.HousekeepingStep {
	next := 0
	return jobs.HousekeepingStep{Name: name, Batch: func(ctx context.Context, limit int) (int, error) {
		l.mu.Lock()
		l.calls = append(l.calls, name)
		l.mu.Unlock()
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > time.Minute {
			return 0, errors.New("the step has no budget of its own within a minute")
		}
		if next >= len(batches) {
			return 0, nil
		}
		batch := batches[next]
		next++
		return batch(limit)
	}}
}

func removes(n int) func(int) (int, error) { return func(int) (int, error) { return n, nil } }

func TestEachStepRunsOnceInOrderWithABatchAndItsOwnBudget(t *testing.T) {
	var l housekeepingLog
	var asked int
	jobs.Housekeep(context.Background(), l.logger(), []jobs.HousekeepingStep{
		l.step("first", func(limit int) (int, error) { asked = limit; return 3, nil }),
		l.step("second", removes(0)),
	})
	if strings.Join(l.calls, ",") != "first,second" {
		t.Errorf("calls = %v, want one call each, in order", l.calls)
	}
	if asked != 500 {
		t.Errorf("a batch was %d, want 500", asked)
	}
	for _, want := range []string{`"step":"first","removed":3`, `"step":"second","removed":0`} {
		if !strings.Contains(l.text(), want) {
			t.Errorf("the log is %s, want it to hold %s", l.text(), want)
		}
	}
}

func TestAStepRepeatsWhileItsBatchesComeBackFull(t *testing.T) {
	var l housekeepingLog
	jobs.Housekeep(context.Background(), l.logger(), []jobs.HousekeepingStep{
		l.step("full", removes(500), removes(500), removes(120), removes(7)),
	})
	if len(l.calls) != 3 {
		t.Errorf("%d batches ran, want 3: two full and the short one that ends the step", len(l.calls))
	}
	if !strings.Contains(l.text(), `"removed":1120`) {
		t.Errorf("the log is %s, want the 1120 removed", l.text())
	}
}

func TestAStepThatNeverCatchesUpStopsAfterTwentyBatchesAndSaysSo(t *testing.T) {
	var l housekeepingLog
	forever := make([]func(int) (int, error), 100)
	for i := range forever {
		forever[i] = removes(500)
	}
	jobs.Housekeep(context.Background(), l.logger(), []jobs.HousekeepingStep{l.step("endless", forever...)})
	if len(l.calls) != 20 {
		t.Errorf("%d batches ran, want the cap, 20", len(l.calls))
	}
	if !strings.Contains(l.text(), "left work for the next run") || !strings.Contains(l.text(), `"removed":10000`) {
		t.Errorf("the log is %s, want it to say work is left and 10000 went", l.text())
	}
}

func TestAFailedStepIsLoggedAndTheNextOneStillRuns(t *testing.T) {
	var l housekeepingLog
	jobs.Housekeep(context.Background(), l.logger(), []jobs.HousekeepingStep{
		l.step("broken", removes(500), func(int) (int, error) { return 40, errors.New("the bucket is away") }, removes(500)),
		l.step("fine", removes(1)),
	})
	if strings.Join(l.calls, ",") != "broken,broken,fine" {
		t.Errorf("calls = %v, want the failed step to stop at its error and the next to run", l.calls)
	}
	for _, want := range []string{`"code":"HOUSEKEEPING_STEP_FAILED"`, `"step":"broken","removed":540`, `"failure":"error"`, `"step":"fine","removed":1`} {
		if !strings.Contains(l.text(), want) {
			t.Errorf("the log is %s, want it to hold %s", l.text(), want)
		}
	}
}

func TestAStopTellsTheRemainingStepsNotToStart(t *testing.T) {
	var l housekeepingLog
	ctx, cancel := context.WithCancel(context.Background())
	jobs.Housekeep(ctx, l.logger(), []jobs.HousekeepingStep{
		l.step("first", func(int) (int, error) { cancel(); return 1, nil }),
		l.step("second", removes(1)),
	})
	if strings.Join(l.calls, ",") != "first" {
		t.Errorf("calls = %v, want only the step that was running", l.calls)
	}
}

func TestTheHousekeepingRunsRightAfterEachRefreshTokenPruneEvenWhenThePruneFailed(t *testing.T) {
	for name, outcome := range map[string]error{"the prune worked": nil, "the prune failed": errors.New("the database is away")} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var mu sync.Mutex
			var order []string
			app := &identityapp.Application{Commands: identityapp.Commands{
				PruneExpiredTokens: cqrs.HandlerFunc[identitycommand.PruneExpiredTokens, int64](func(context.Context, identitycommand.PruneExpiredTokens) (int64, error) {
					mu.Lock()
					defer mu.Unlock()
					order = append(order, "prune")
					return 0, outcome
				}),
			}}
			done := make(chan struct{})
			ran := make(chan struct{}, 4)
			go func() {
				jobs.PruneRefreshTokens(ctx, slog.New(slog.DiscardHandler), app, func(context.Context) {
					mu.Lock()
					order = append(order, "housekeeping")
					mu.Unlock()
					ran <- struct{}{}
				})
				close(done)
			}()
			select {
			case <-ran:
			case <-time.After(2 * time.Second):
				t.Fatal("the housekeeping did not run at start")
			}
			select {
			case <-ran:
				t.Fatal("the housekeeping ran again within its day")
			case <-time.After(100 * time.Millisecond):
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("the job kept running after the process was told to stop")
			}
			mu.Lock()
			defer mu.Unlock()
			if strings.Join(order, ",") != "prune,housekeeping" {
				t.Errorf("order = %v, want the prune and then the housekeeping, once", order)
			}
		})
	}
}

func TestTheDailyJobsTickOnceADay(t *testing.T) {
	if jobs.PruneInterval != 24*time.Hour {
		t.Errorf("PruneInterval = %v, want 24h", jobs.PruneInterval)
	}
}

func TestTheHousekeepingRunsAgainOnEachTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &identityapp.Application{Commands: identityapp.Commands{
		PruneExpiredTokens: cqrs.HandlerFunc[identitycommand.PruneExpiredTokens, int64](func(context.Context, identitycommand.PruneExpiredTokens) (int64, error) {
			return 0, nil
		}),
	}}
	ran := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		jobs.PruneRefreshTokensEvery(ctx, slog.New(slog.DiscardHandler), app, 10*time.Millisecond, func(context.Context) { ran <- struct{}{} })
		close(done)
	}()
	for i := range 3 {
		select {
		case <-ran:
		case <-time.After(2 * time.Second):
			t.Fatalf("the housekeeping ran %d times, want the start-up run and a run on each tick", i)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the job kept running after the process was told to stop")
	}
}

type statusError int

func (e statusError) Error() string {
	return fmt.Sprintf("delete https://objects.example/quizzivy-imports/exports/0198f000.zip: status %d", int(e))
}
func (e statusError) HTTPStatusCode() int { return int(e) }

type unansweredError struct{ cause error }

func (e unansweredError) Error() string {
	return "operation error S3: ListObjectsV2, " + e.cause.Error()
}
func (e unansweredError) Unwrap() error       { return e.cause }
func (e unansweredError) HTTPStatusCode() int { return 0 }

func TestAFailureIsLoggedAsAClassAndNeverAsTheErrorsText(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"a plain error":                      {errors.New("exports/0198f000.zip is locked"), `"failure":"error"`},
		"a deadline":                         {fmt.Errorf("list branding/ab12: %w", context.DeadlineExceeded), `"failure":"timeout"`},
		"a cancellation":                     {fmt.Errorf("delete: %w", context.Canceled), `"failure":"canceled"`},
		"a database error":                   {fmt.Errorf("prune: %w", &pgconn.PgError{Code: "42501", Message: "permission denied for table user_activity_days"}), `"failure":"database 42501"`},
		"a storage status":                   {fmt.Errorf("sweep logos: %w", statusError(403)), `"failure":"storage http 403"`},
		"a storage error without a response": {fmt.Errorf("list: %w", unansweredError{&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}}), `"failure":"network"`},
		"a refused dial": {&url.Error{Op: "Delete", URL: "http://localhost:19999/quizzivy-imports/exports/0198f000.zip", Err: &net.OpError{
			Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}}, `"failure":"network"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var l housekeepingLog
			jobs.Housekeep(context.Background(), l.logger(), []jobs.HousekeepingStep{
				l.step("broken", func(int) (int, error) { return 0, c.err }),
			})
			text := l.text()
			if !strings.Contains(text, c.want) {
				t.Errorf("the log is %s, want it to hold %s", text, c.want)
			}
			for _, leak := range []string{"0198f000", "quizzivy-imports", "localhost", "branding", "user_activity_days", "permission denied", "objects.example"} {
				if strings.Contains(text, leak) {
					t.Errorf("the log is %s, and it carries %q from the error", text, leak)
				}
			}
		})
	}
}
