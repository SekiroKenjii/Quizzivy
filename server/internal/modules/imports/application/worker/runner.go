// Package worker executes one leased private import at a time, fencing progress and results after cancellation or takeover.
package worker

import (
	"context"
	"errors"
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"time"
)

// Processor produces a private machine candidate and must stop when its context is cancelled; it never publishes an assessment.
type Processor interface {
	Process(context.Context, domain.Run, func(string) error) (domain.Outcome, error)
}

const processorOutputInvalid = "PROCESSOR_OUTPUT_INVALID"

const processingFailed = "PROCESSING_FAILED"
const processingTimeout = "PROCESSING_TIMEOUT"

// Failure classifies processing without including document contents, provider responses or signed URLs in operational events.
type Failure struct {
	Code      string
	Retryable bool
}

func (e Failure) Error() string { return e.Code }

// Event contains safe operational identifiers and timing only.
type Event struct {
	Kind, RunID, ImportID, Stage, Code string
	Attempt                            int
	Duration                           time.Duration
}

// Runner executes a single claim; a supervisor controls polling and process-level resource limits.
type Runner struct {
	Queue                               domain.Queue
	Processor                           Processor
	Policy                              domain.ClaimPolicy
	HeartbeatEvery, Timeout, RetryAfter time.Duration
	Observe                             func(Event)
}

func (r Runner) RunOne(ctx context.Context) (bool, error) {
	if r.Queue == nil || r.Processor == nil || r.HeartbeatEvery <= 0 || r.HeartbeatEvery >= r.Policy.Lease || r.Timeout <= 0 {
		return false, errors.New("imports: invalid worker configuration")
	}
	run, err := r.Queue.Claim(ctx, r.Policy)
	if errors.Is(err, domain.ErrNoWork) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	started := time.Now()
	r.emit(run, "claimed", "", 0)
	result, processErr := r.execute(ctx, run)
	err = r.finish(ctx, run, result, processErr)
	if errors.Is(err, domain.ErrLeaseLost) {
		r.emit(run, "lease_lost", "WORKER_LEASE_LOST", time.Since(started))
		return true, nil
	}
	code := ""
	kind := "completed"
	if err != nil {
		kind = "write_failed"
		code = "WORKER_STATE_WRITE_FAILED"
	} else if processErr != nil {
		kind = "stopped"
		code = classify(processErr).Code
	}
	r.emit(run, kind, code, time.Since(started))
	return true, err
}
func (r Runner) execute(ctx context.Context, run domain.Run) (domain.Outcome, error) {
	work, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	done, stopped := make(chan struct{}), make(chan struct{})
	heartbeatError := make(chan error, 1)
	progressError := make(chan error, 1)
	go func() { defer close(stopped); r.heartbeat(work, run.Claim(), done, heartbeatError, cancel) }()
	result, err := r.Processor.Process(work, run, r.progress(work, run, progressError, cancel))
	close(done)
	<-stopped
	select {
	case leaseErr := <-heartbeatError:
		return domain.Outcome{}, leaseErr
	default:
	}
	if err == nil && ctx.Err() != nil && domain.ValidateRunResult(result.Result) == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return domain.Outcome{}, Failure{Code: interrupted, Retryable: true}
	}
	if errors.Is(work.Err(), context.DeadlineExceeded) {
		return domain.Outcome{}, Failure{Code: processingTimeout, Retryable: true}
	}
	select {
	case progressErr := <-progressError:
		return domain.Outcome{}, progressErr
	default:
	}

	if work.Err() != nil {
		return domain.Outcome{}, Failure{Code: processingTimeout, Retryable: true}
	}
	if err == nil && domain.ValidateRunResult(result.Result) != nil {
		return domain.Outcome{}, Failure{Code: processorOutputInvalid, Retryable: false}
	}
	return result, err
}
func (r Runner) progress(ctx context.Context, run domain.Run, failures chan<- error, cancel context.CancelFunc) func(string) error {
	return func(stage string) error {
		err := r.Queue.Progress(ctx, run.Claim(), stage)
		if err != nil {
			select {
			case failures <- err:
			default:
			}
			cancel()
		}
		if err == nil {
			event := Event{Kind: "stage", RunID: run.ID, ImportID: run.ImportID, Stage: stage, Attempt: run.AttemptCount}
			if r.Observe != nil {
				r.Observe(event)
			}
		}
		return err
	}
}

func (r Runner) heartbeat(ctx context.Context, claim domain.Claim, done <-chan struct{}, failures chan<- error, cancel context.CancelFunc) {
	ticker := time.NewTicker(r.HeartbeatEvery)
	defer ticker.Stop()
	renewed := time.Now()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			pulse, cancelPulse := context.WithTimeout(ctx, r.HeartbeatEvery)
			err := r.Queue.Heartbeat(pulse, claim, r.Policy.Lease)
			cancelPulse()
			switch {
			case err == nil:
				renewed = time.Now()
			case errors.Is(err, domain.ErrLeaseLost) || time.Since(renewed) >= r.Policy.Lease-r.HeartbeatEvery:
				failures <- err
				cancel()
				return
			}
		}
	}
}
func (r Runner) finish(ctx context.Context, run domain.Run, result domain.Outcome, processErr error) error {
	if errors.Is(processErr, domain.ErrLeaseLost) {
		return domain.ErrLeaseLost
	}
	write, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if processErr == nil {
		err := r.Queue.Complete(write, run.Claim(), result)
		if errors.Is(err, domain.ErrLeaseLost) {
			return domain.ErrLeaseLost
		}
		return err
	}
	failure := classify(processErr)
	err := r.Queue.Fail(write, domain.RunFailure{Claim: run.Claim(), Code: failure.Code, Retryable: failure.Retryable, Released: failure.Code == interrupted, RetryAfter: r.RetryAfter})
	if errors.Is(err, domain.ErrLeaseLost) {
		return domain.ErrLeaseLost
	}
	return err
}

const interrupted = "WORKER_INTERRUPTED"

func classify(err error) Failure {
	var pointer *Failure
	if errors.As(err, &pointer) && pointer != nil {
		return safeFailure(*pointer)
	}
	var failure Failure
	if errors.As(err, &failure) {
		return safeFailure(failure)
	}
	if errors.Is(err, domain.ErrLeaseLost) {
		return Failure{Code: "WORKER_LEASE_LOST"}
	}
	return Failure{Code: processingFailed, Retryable: true}
}
func (r Runner) emit(run domain.Run, kind, code string, duration time.Duration) {
	if r.Observe != nil {
		r.Observe(Event{Kind: kind, RunID: run.ID, ImportID: run.ImportID, Stage: run.Stage, Code: code, Attempt: run.AttemptCount, Duration: duration})
	}
}

func safeFailure(f Failure) Failure {
	if !slices.Contains([]string{processingFailed, processorOutputInvalid, "SOURCE_INVALID", "SOURCE_TOO_LARGE", "SOURCE_UNSUPPORTED", "STORAGE_UNAVAILABLE", "STORAGE_QUOTA_EXCEEDED", "STORAGE_INTEGRITY_FAILED", "PROVIDER_UNAVAILABLE", "PROVIDER_RATE_LIMITED", "PROVIDER_OUTPUT_INVALID", "CONVERSION_FAILED", "CONVERSION_TIMEOUT", "CONVERSION_BUSY", "CONVERSION_CLEANUP_FAILED", "LEGACY_CONVERSION_UNAVAILABLE", processingTimeout, interrupted, "WORKER_LEASE_LOST"}, f.Code) {
		f.Code = processingFailed
	}
	return f
}
