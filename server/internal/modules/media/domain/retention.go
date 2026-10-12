package domain

import (
	"context"
	"time"
)

// RetiredFileGrace is how long a replaced or deleted file's object stays in
// storage after nothing names it. The file is then removed for good: nothing
// restores it.
const RetiredFileGrace = 7 * 24 * time.Hour

// Retired is a file that has left the library, because it was replaced or
// deleted, and what keeps it in storage. Asset is its row as the library held
// it, with its URL once the application has signed one. UsedIn names the
// published versions that still use it. DeleteAt is when its object is removed
// when nothing names it now, and nil while something does: the removal is then
// RetiredFileGrace after the last of them stops.
type Retired struct {
	Asset    Asset
	UsedIn   []TestRef
	DeleteAt *time.Time
}

// PurgeOutcome is what one attempt to remove a retired file's object came to.
type PurgeOutcome uint8

const (
	// PurgeSkipped means the file was not due when the attempt looked: another
	// sweep holds it or finished it, or it was bound, purged or changed since
	// the sweep listed it.
	PurgeSkipped PurgeOutcome = iota
	// PurgeRemoved means the object is gone from storage and the row is marked.
	PurgeRemoved
	// PurgeNamed means something names the file again, so its clock was cleared
	// and its object left alone.
	PurgeNamed
)

// PurgeInput is one removal. Due is the latest unreferenced_at a file may carry
// to be removed now. Remove deletes the object under the file's key, and it
// runs while the file's row is locked: a failure leaves the row as it was, to be
// found by the next sweep.
type PurgeInput struct {
	ID     string
	Due    time.Time
	Now    time.Time
	Remove func(ctx context.Context, storageKey string) error
}

// Observed counts what one look at the retired files changed: files first found
// to be named by nothing, whose seven days start then, and files found named
// again, whose wait is cancelled.
type Observed struct {
	Unreferenced int
	Named        int
}

// Retention is the storage side of a retired file's life, which the sweep
// drives. A retired file is one that is deleted or replaced and whose object is
// still in storage.
type Retention interface {
	Observe(ctx context.Context, now time.Time) (Observed, error)
	Due(ctx context.Context, due time.Time, after string, limit int) ([]string, error)
	Purge(ctx context.Context, in PurgeInput) (PurgeOutcome, error)
}
