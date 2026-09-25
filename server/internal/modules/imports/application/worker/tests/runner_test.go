package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"sync"
	"testing"
	"time"
)

type queue struct {
	domain.Queue
	mu                                     sync.Mutex
	heartbeatErr, completeErr, progressErr error
	completions                            int
	failure                                *domain.RunFailure
}

func (q *queue) Claim(context.Context, domain.ClaimPolicy) (domain.Run, error) {
	id := "worker"
	return domain.Run{ID: "run", ImportID: "import", WorkerID: &id, ClaimToken: 1, SourceRevision: 2, AttemptCount: 1, MaxAttempts: 3}, nil
}
func (q *queue) Heartbeat(context.Context, domain.Claim, time.Duration) error { return q.heartbeatErr }
func (q *queue) Progress(context.Context, domain.Claim, string) error         { return q.progressErr }
func (q *queue) Complete(_ context.Context, c domain.Claim, outcome domain.Outcome) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.completions++
	if c.Token != 1 || c.SourceRevision != 2 || string(outcome.Result) != `{"schemaVersion":1}` {
		return errors.New("wrong result identity")
	}
	return q.completeErr
}
func (q *queue) Fail(_ context.Context, in domain.RunFailure) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.failure = &in
	return nil
}

type processor func(context.Context, domain.Run, func(string) error) (json.RawMessage, error)

func (p processor) Process(ctx context.Context, r domain.Run, progress func(string) error) (domain.Outcome, error) {
	result, err := p(ctx, r, progress)
	return domain.Outcome{Result: result, Draft: json.RawMessage(`{}`)}, err
}
func runner(q *queue, p processor) worker.Runner {
	return worker.Runner{Queue: q, Processor: p, Policy: domain.ClaimPolicy{Lease: time.Second}, HeartbeatEvery: time.Millisecond, Timeout: time.Second, RetryAfter: time.Second}
}

func TestLeaseLossCancelsProcessingAndCannotCompleteOrFailNewOwner(t *testing.T) {
	q := &queue{heartbeatErr: domain.ErrLeaseLost}
	r := runner(q, func(ctx context.Context, _ domain.Run, _ func(string) error) (json.RawMessage, error) {
		<-ctx.Done()
		return json.RawMessage(`{"schemaVersion":1}`), nil
	})
	var events []worker.Event
	r.Observe = func(e worker.Event) { events = append(events, e) }
	worked, err := r.RunOne(context.Background())
	if err != nil || !worked || q.completions != 0 || q.failure != nil {
		t.Fatalf("late worker wrote state: worked=%v error=%v complete=%d", worked, err, q.completions)
	}
	if len(events) != 2 || events[1].Kind != "lease_lost" {
		t.Fatalf("lease loss reported success: %+v", events)
	}
}
func TestProgressConflictCancelsEvenAnExecutorThatIgnoresItsError(t *testing.T) {
	q := &queue{progressErr: domain.ErrLeaseLost}
	r := runner(q, func(ctx context.Context, _ domain.Run, progress func(string) error) (json.RawMessage, error) {
		_ = progress("extraction")
		<-ctx.Done()
		return json.RawMessage(`{"schemaVersion":1}`), nil
	})
	if _, err := r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.completions != 0 || q.failure != nil {
		t.Fatal("progress failure did not fence result")
	}
}
func TestLateCompletionIsObservedAsLeaseLoss(t *testing.T) {
	q := &queue{completeErr: domain.ErrLeaseLost}
	r := runner(q, func(context.Context, domain.Run, func(string) error) (json.RawMessage, error) {
		return json.RawMessage(`{"schemaVersion":1}`), nil
	})
	var events []worker.Event
	r.Observe = func(e worker.Event) { events = append(events, e) }
	if _, err := r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if events[len(events)-1].Kind != "lease_lost" {
		t.Fatal("rejected result was reported as completed")
	}
}
func TestFailureEventsNeverEchoProcessorDocumentText(t *testing.T) {
	q := &queue{}
	r := runner(q, func(context.Context, domain.Run, func(string) error) (json.RawMessage, error) {
		return nil, &worker.Failure{Code: "PRIVATE_ANSWER_EVIDENCE", Retryable: false}
	})
	var events []worker.Event
	r.Observe = func(e worker.Event) { events = append(events, e) }
	if _, err := r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.failure == nil || q.failure.Code != "PROCESSING_FAILED" || q.failure.Retryable {
		t.Fatalf("failure classification: %+v", q.failure)
	}
	if events[len(events)-1].Code != "PROCESSING_FAILED" {
		t.Fatal("processor text escaped into observability")
	}
}
func TestGracefulShutdownReleasesClaimForBoundedRetry(t *testing.T) {
	q := &queue{}
	ctx, cancel := context.WithCancel(context.Background())
	r := runner(q, func(ctx context.Context, _ domain.Run, _ func(string) error) (json.RawMessage, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if _, err := r.RunOne(ctx); err != nil {
		t.Fatal(err)
	}
	if q.failure == nil || q.failure.Code != "WORKER_INTERRUPTED" || !q.failure.Retryable || q.completions != 0 {
		t.Fatalf("shutdown lost state: %+v", q.failure)
	}
}

func TestMalformedProcessorOutputFailsWithoutClaimingSuccess(t *testing.T) {
	q := &queue{}
	r := runner(q, func(context.Context, domain.Run, func(string) error) (json.RawMessage, error) {
		return json.RawMessage(`[]`), nil
	})
	var events []worker.Event
	r.Observe = func(e worker.Event) { events = append(events, e) }
	if _, err := r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.completions != 0 || q.failure == nil || q.failure.Retryable || q.failure.Code != "PROCESSOR_OUTPUT_INVALID" {
		t.Fatal("malformed output reached completion")
	}
	if events[len(events)-1].Kind == "completed" {
		t.Fatal("malformed output reported success")
	}
}

func TestATransientHeartbeatErrorDoesNotAbandonAValidLease(t *testing.T) {
	q := &queue{heartbeatErr: errors.New("pool busy")}
	r := runner(q, func(ctx context.Context, _ domain.Run, _ func(string) error) (json.RawMessage, error) {
		time.Sleep(20 * time.Millisecond)
		return json.RawMessage(`{"schemaVersion":1}`), ctx.Err()
	})
	if _, err := r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if q.completions != 1 || q.failure != nil {
		t.Fatalf("completions %d failure %+v", q.completions, q.failure)
	}
}

func TestShutdownReleasesTheRunWithoutSpendingAnAttempt(t *testing.T) {
	q := &queue{}
	ctx, stop := context.WithCancel(context.Background())
	r := runner(q, func(work context.Context, _ domain.Run, _ func(string) error) (json.RawMessage, error) {
		stop()
		<-work.Done()
		return nil, work.Err()
	})
	_, _ = r.RunOne(ctx)
	if q.failure == nil || !q.failure.Released || q.failure.Code != "WORKER_INTERRUPTED" {
		t.Fatalf("failure %+v", q.failure)
	}
}
