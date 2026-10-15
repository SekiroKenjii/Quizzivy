package domain

import (
	"context"
	"errors"
	"strconv"
	"time"
)

const (
	// ImportBatchSize is how many rows the worker settles in one transaction.
	ImportBatchSize = 25
	// ImportLease is how long a claim holds a job; a job still running after it
	// is claimed again.
	ImportLease = 30 * time.Minute
	// ImportRunLimit is the longest one claim runs before it gives the job back.
	// It is shorter than ImportLease, so a job is never run by two workers.
	ImportRunLimit = 25 * time.Minute
	// ImportMaxAttempts is how many times a job is claimed before it is failed
	// rather than tried again.
	ImportMaxAttempts = 3
	// SheetLifetime is how long a finished job's sheet can be taken.
	SheetLifetime = 24 * time.Hour
	// UserImportsKept is how long a job, with its rows, stays in the tables
	// once it has finished, done or failed. The rows keep each person's name and
	// address as the file spelled them, so they go with the job.
	UserImportsKept = 30 * 24 * time.Hour
)

// The error codes a failed job carries.
const (
	// ImportFailedNotAllowed says the importer's account, or a right the file
	// needs of it, is gone: the account is disabled, or no longer holds
	// people.users.manage, a role the file names or scope.all for a class.
	ImportFailedNotAllowed = "IMPORTER_NOT_ALLOWED"
	// ImportFailedDelivery says the way the accounts were to be given a
	// password became unavailable while the job ran.
	ImportFailedDelivery = "DELIVERY_UNAVAILABLE"
	// ImportFailedInterrupted says the worker claimed the job
	// ImportMaxAttempts times and none of the runs finished it.
	ImportFailedInterrupted = "WORKER_INTERRUPTED"
)

var (
	// ErrNoImportWork is what Claim answers when no job is waiting.
	ErrNoImportWork = errors.New("user import: no job is waiting")
	// ErrImportLeaseLost is a write to a job the worker no longer holds: it
	// was claimed again, or it is no longer running.
	ErrImportLeaseLost = errors.New("user import: the claim on the job was lost")
	// ErrSheetUnavailable is a hand-out sheet asked for when the job has none
	// to give: it is not done, it was delivered by email, it created no
	// account, the sheet was downloaded, or it expired.
	ErrSheetUnavailable = errors.New("user import: there is no sheet to hand out")
)

// SheetAAD is the data a row's sealed password is bound to: the job and the
// row's ordinal, so a value cannot be moved to another row or job.
func SheetAAD(jobID string, ordinal int) []byte {
	return []byte(jobID + ":" + strconv.Itoa(ordinal))
}

// ImportTodo is a stored row that waits for its account.
type ImportTodo struct {
	Ordinal  int
	FullName string
	Email    string
	RoleID   string
	ClassID  string
}

// ImportCreate is a row with what its account needs: the role as it was read,
// and for a sheet the password's hash and the sealed password.
type ImportCreate struct {
	ImportTodo
	RoleName     string
	RoleRevision int64
	Hash         string
	Secret       []byte
}

// ImportBatch is rows to settle for the job under the claim Attempt, on behalf
// of By. Keep says the temporary passwords need not be changed at the first
// sign-in.
type ImportBatch struct {
	JobID    string
	Attempt  int
	By       string
	Delivery ImportDelivery
	Keep     bool
	Now      time.Time
	Rows     []ImportCreate
}

// ImportProgress is what a settled batch did, and whether it finished the job.
type ImportProgress struct {
	Created int
	Skipped int
	Done    bool
}

// SheetRow is one line of the hand-out sheet.
type SheetRow struct {
	FullName string
	Email    string
	Role     string
	Class    string
	Password string
}

// SecretOpener opens the sealed password of the row at ordinal.
type SecretOpener func(ordinal int, sealed []byte) (string, error)

// UserImportQueue is the worker's side of the user imports.
type UserImportQueue interface {
	// Claim takes the oldest waiting job: a queued one, or one still running
	// after the lease. A job already claimed ImportMaxAttempts times is
	// failed with ImportFailedInterrupted instead. It answers ErrNoImportWork
	// when there is none.
	Claim(ctx context.Context) (ImportJob, error)
	// NextDue says when a job next falls due, false when none waits.
	NextDue(ctx context.Context) (time.Time, bool, error)
	// Todo returns up to limit rows of the job that wait for their account, in
	// order.
	Todo(ctx context.Context, jobID string, limit int) ([]ImportTodo, error)
	// Settle creates the accounts of batch in one transaction, settles their
	// rows and moves the job's counters; the job is done when they reach its
	// total. issue, when not nil, gives each new account its invite on the
	// same transaction. It fails with ErrImportLeaseLost when the job is not
	// running under batch.Attempt, and with ErrRoleChanged, having changed
	// nothing, when a role is no longer at the revision the row carries.
	Settle(ctx context.Context, batch ImportBatch, issue LinkIssue) (ImportProgress, error)
	// Fail ends the job with code and erases the passwords sealed for its
	// sheet, which nothing can hand out any more.
	Fail(ctx context.Context, jobID string, attempt int, code string) error
	// Retry gives the job back to the queue, the attempt spent.
	Retry(ctx context.Context, jobID string, attempt int) error
	// Release gives the job back to the queue and the attempt with it.
	Release(ctx context.Context, jobID string, attempt int) error
}
