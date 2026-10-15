// Package ports declares what the exports need from outside the module: the
// private object store, the worker's wake signal and the writer of a table.
package ports

import (
	"context"
	"io"
	"time"
)

// Store is the private bucket the finished ZIPs wait in.
type Store interface {
	// Put stores size bytes of body under key.
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	// SignedDownloadURL authorises key as an attachment named filename for ttl.
	SignedDownloadURL(ctx context.Context, key, filename string, ttl time.Duration) (string, error)
}

// Wake tells the worker that an export is waiting. It never blocks.
type Wake interface {
	Wake()
}

// Table writes the rows of one file for a spreadsheet to open: its cells are
// escaped so that none runs as a formula.
type Table interface {
	Row(cells ...string) error
	Flush() error
}

// NewTable starts a table on w.
type NewTable func(w io.Writer) (Table, error)
