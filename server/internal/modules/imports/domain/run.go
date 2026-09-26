package domain

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/shared/actor"
	"time"
)

var (
	ErrNoWork        = errors.New("imports: no eligible work")
	ErrLeaseLost     = errors.New("imports: worker lease lost")
	ErrInvalidResult = errors.New("imports: invalid processing result")
)

// Run pins a processing request to an immutable source set and pipeline version; Result is private candidate evidence, never learner content.
type Run struct {
	ID, ImportID, RequestID, RequestedBy, PipelineVersion, Status, Stage string
	SourceRevision, ExpectedRevision, ClaimToken                         int64
	AttemptCount, MaxAttempts                                            int
	WorkerID, ErrorCode                                                  *string
	LeaseUntil, CompletedAt                                              *time.Time
	AvailableAt, CreatedAt, UpdatedAt                                    time.Time
	Result                                                               json.RawMessage
	Profile                                                              RecognitionProfile
}

type Schedule struct {
	ImportID, RequestID, PipelineVersion string
	ExpectedRevision, SourceRevision     int64
	Actor                                actor.Actor
	MaxAttempts                          int
	Profile                              RecognitionProfile
}
type Cancel struct {
	ImportID         string
	ExpectedRevision int64
	Actor            actor.Actor
}

// Claim binds every worker write to its lease, run identity and immutable source revision.
type Claim struct {
	RunID, ImportID, WorkerID string
	Token, SourceRevision     int64
}

func (r Run) Claim() Claim {
	worker := ""
	if r.WorkerID != nil {
		worker = *r.WorkerID
	}
	return Claim{RunID: r.ID, ImportID: r.ImportID, WorkerID: worker, Token: r.ClaimToken, SourceRevision: r.SourceRevision}
}

// ClaimPolicy limits simultaneous live leases across processes, not merely goroutines in one worker.
type ClaimPolicy struct {
	WorkerID, PipelineVersion string
	Lease                     time.Duration
	GlobalLimit, ActorLimit   int
}

// RunFailure stops a claimed run; Released returns it to the queue without spending an attempt, for a worker that is shutting down.
type RunFailure struct {
	Claim      Claim
	Code       string
	Retryable  bool
	Released   bool
	RetryAfter time.Duration
}

// Queue serializes import transitions before run transitions; expired or superseded claims cannot publish any state or result.
type Queue interface {
	Schedule(context.Context, Schedule) (Run, error)
	Run(context.Context, string, string) (Run, error)
	Runs(context.Context, string) ([]Run, error)
	Events(context.Context, string, string) ([]RunEvent, error)
	Cancel(context.Context, Cancel) (Import, error)
	Claim(context.Context, ClaimPolicy) (Run, error)
	Heartbeat(context.Context, Claim, time.Duration) error
	Progress(context.Context, Claim, string) error
	Complete(context.Context, Claim, Outcome) error
	Fail(context.Context, RunFailure) error
	NextDue(context.Context, string) (time.Time, bool, error)
}

// ValidateRunResult bounds the private result envelope; the processor owns versioned candidate semantics before returning it.
func ValidateRunResult(result json.RawMessage) error {
	if len(result) == 0 || len(result) > 8<<20 || !json.Valid(result) {
		return ErrInvalidResult
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(result, &object); err != nil || object == nil {
		return ErrInvalidResult
	}
	return nil
}

// RunEvent is append-only operational history without document contents or provider response bodies.
type RunEvent struct {
	ID, RunID, ImportID, Kind, Stage string
	ClaimToken                       int64
	AttemptCount                     int
	WorkerID, ActorID, ErrorCode     *string
	CreatedAt                        time.Time
}
