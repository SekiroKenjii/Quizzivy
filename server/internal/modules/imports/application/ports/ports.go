// Package ports defines the private storage and bounded inspection used by import intake.
package ports

import (
	"context"
	"io"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"
	"time"
)

type ObjectStore interface {
	Put(context.Context, string, string, io.Reader, int64) error
	SignedDownloadURL(context.Context, string, string, time.Duration) (string, error)
}

// ArtifactStore persists checksum-verified private objects with create-only semantics and streams them only to trusted processors.
type ArtifactStore interface {
	PutImmutable(context.Context, string, string, io.ReadSeeker, int64, []byte) error
	Open(context.Context, string) (io.ReadCloser, int64, error)
}

// Inspector validates a native Word package without external requests or returning source content.
type Inspector interface {
	Inspect(context.Context, io.ReaderAt, int64) error
}

// Runs queues and stops processing for the API; the worker owns every other run transition.
type Runs interface {
	Schedule(context.Context, domain.Schedule) (domain.Run, error)
	Cancel(context.Context, domain.Cancel) (domain.Import, error)
	DraftRun(context.Context, string) (domain.Run, error)
}

// Materializer creates the plan's draft test and runs record in the same transaction, returning the test ID;
// nothing persists unless both succeed.
type Materializer interface {
	Materialize(ctx context.Context, plan domain.CommitPlan, by actor.Actor, record func(context.Context, domain.CommitStore, string) error) (string, error)
}
