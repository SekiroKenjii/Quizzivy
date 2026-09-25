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
	ObjectRemover
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

// Retention finds imports whose files the retention policy no longer keeps
// and records their removal. The caller removes the objects in between, so an
// import is marked only once its bytes are gone.
type Retention interface {
	CloseIdle(context.Context, time.Time, int) ([]string, error)
	ExpiredFiles(context.Context, time.Time, time.Time, domain.Cursor, int) ([]domain.Cursor, error)
	FilesOf(context.Context, string) ([]string, error)
	FilesRemoved(context.Context, string) error
}

// ObjectRemover deletes private objects; deleting one already gone succeeds.
type ObjectRemover interface {
	Delete(context.Context, string) error
}

// WorkerSignal tells the import worker that a run was queued. Wake never blocks
// and never fails the request; a lost signal only delays the run until the
// worker's next scheduled check.
type WorkerSignal interface {
	Wake()
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
