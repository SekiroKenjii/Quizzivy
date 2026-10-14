// Package domain is the data export's model: what an export is and the states
// it passes through, the files its ZIP holds with the columns of each, and the
// ports the worker builds it through. An Admin asks for an export, the worker
// writes one ZIP of CSV files to the private bucket, and the requester fetches
// it once from an app page; nothing here knows a database or an object store.
package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Status is where an export stands.
type Status string

// The states of an export. Queued becomes Running when the worker claims it,
// and Running becomes Ready or Failed. A Ready export becomes Expired once its
// time has passed, whether or not its file has been removed yet.
const (
	Queued  Status = "queued"
	Running Status = "running"
	Ready   Status = "ready"
	Failed  Status = "failed"
	Expired Status = "expired"
)

// The limits an export lives within.
const (
	// Lease is how long a claimed export may stay Running before another
	// claim takes it again. A build is cut off before it, at Timeout.
	Lease = 30 * time.Minute
	// Timeout is the longest a build may run.
	Timeout = 25 * time.Minute
	// MaxAttempts is how many times an export is claimed before it is failed
	// instead of retried.
	MaxAttempts = 3
	// KeepFor is how long a Ready export can be fetched.
	KeepFor = 24 * time.Hour
	// URLTTL is how long the address a fetch returns works.
	URLTTL = 60 * time.Second
	// RequestInterval is how long an Admin's export that has not failed keeps
	// them from asking for another.
	RequestInterval = time.Hour
)

// The codes a failed export carries. A code is never a message: nothing about
// the data reaches the table.
const (
	CodeFailed      = "EXPORT_FAILED"
	CodeTimeout     = "EXPORT_TIMEOUT"
	CodeStorage     = "STORAGE_UNAVAILABLE"
	CodeInterrupted = "WORKER_INTERRUPTED"
)

// Errors a repository answers with.
var (
	// ErrInProgress means another export is queued or running.
	ErrInProgress = errors.New("exports: an export is already queued or running")
	// ErrNotFound means no export of the caller has the id. Another user's
	// export answers exactly as a missing one does.
	ErrNotFound = errors.New("exports: no such export")
	// ErrUnavailable means the export exists but cannot be fetched: it is not
	// ready, has expired, failed, or was fetched already.
	ErrUnavailable = errors.New("exports: the export cannot be fetched")
	// ErrNoWork means no export is waiting for the worker.
	ErrNoWork = errors.New("exports: no export is waiting")
	// ErrLeaseLost means the export was taken from the worker that asked to
	// finish it, because its lease lapsed and another claim took it.
	ErrLeaseLost = errors.New("exports: the export is no longer this worker's")
)

// TooSoonError means the actor has an export, created less than
// RequestInterval ago, that has not failed. RetryAfter is how long until the
// newest of them is RequestInterval old.
type TooSoonError struct {
	RetryAfter time.Duration
}

func (e *TooSoonError) Error() string {
	return fmt.Sprintf("exports: an export was requested less than %s ago; retry in %s", RequestInterval, e.RetryAfter)
}

// Export is one request for the centre's data and what became of it.
type Export struct {
	ID             string
	RequestedBy    string
	IncludeAnswers bool
	Status         Status
	Attempts       int
	Bytes          *int64
	CreatedAt      time.Time
	CompletedAt    *time.Time
	ExpiresAt      *time.Time
	DownloadedAt   *time.Time
}

// Request is an Admin's ask for an export.
type Request struct {
	ActorID        string
	IP             string
	UserAgent      string
	IncludeAnswers bool
}

// Fetch is the requester's one fetch of an export.
type Fetch struct {
	ID        string
	ActorID   string
	IP        string
	UserAgent string
}

// Completion is what a worker reports once the file is stored.
type Completion struct {
	ID         string
	Attempt    int
	StorageKey string
	Bytes      int64
}

// Signer turns the object key of an export created at created into the
// address a fetch returns.
type Signer func(key string, created time.Time) (string, error)

// StorageKey is where the file of the export id is kept in the private bucket.
func StorageKey(id string) string { return "exports/" + id + ".zip" }

// Repository is the API's side of the exports.
type Repository interface {
	// Request queues an export in one transaction that audits it as
	// data_export.requested. It fails with a *TooSoonError while the actor has
	// an export that has not failed from the last RequestInterval, and with
	// ErrInProgress while any other export is queued or running. A refused
	// request leaves nothing behind, so it does not count toward the interval.
	Request(ctx context.Context, in Request) (Export, error)
	// Get reads the export id of requester. A Ready export past its time reads
	// Expired. Anyone else's export is ErrNotFound.
	Get(ctx context.Context, id, requester string) (Export, error)
	// Fetch marks the export fetched and returns the signed address of its
	// file, in one transaction that audits it as data_export.downloaded. It
	// fails with ErrNotFound for anyone else's export and ErrUnavailable
	// unless the export is Ready, unexpired and not fetched yet. When sign
	// fails nothing is marked.
	Fetch(ctx context.Context, in Fetch, sign Signer) (string, error)
}

// Queue is the worker's side of the exports.
type Queue interface {
	// Claim takes the oldest export that is Queued, or Running past Lease,
	// and counts the attempt. An export Running past Lease that has been
	// claimed MaxAttempts times is failed with CodeInterrupted instead. It
	// returns ErrNoWork when nothing waits.
	Claim(ctx context.Context) (Export, error)
	// NextDue says when the next export will be due: now for a Queued one, the
	// end of the lease for a Running one. False means none is waiting.
	NextDue(ctx context.Context) (time.Time, bool, error)
	// Complete marks the claimed export Ready for KeepFor, and in the same
	// transaction asks the requester's mailbox to tell them. It returns
	// ErrLeaseLost when the claim is no longer the worker's.
	Complete(ctx context.Context, done Completion) error
	// Fail marks the claimed export Failed with code.
	Fail(ctx context.Context, id string, attempt int, code string) error
	// Release gives the claimed export back to the queue without spending the
	// attempt, for a worker that is shutting down.
	Release(ctx context.Context, id string, attempt int) error
}

// Snapshot reads the tables of one consistent moment.
type Snapshot interface {
	// Rows streams the rows of file, each as one text cell for each of
	// file's columns in order, NULL as the empty string, to emit. It stops at
	// the first error emit returns. It fails when the query does not return
	// the columns of file.
	Rows(ctx context.Context, file File, includeAnswers bool, emit func(cells []string) error) error
}

// Source opens a Snapshot.
type Source interface {
	// Read runs fn over one read-only snapshot, so that every file of an export
	// describes the same moment.
	Read(ctx context.Context, fn func(Snapshot) error) error
}
