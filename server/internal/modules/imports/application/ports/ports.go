// Package ports defines the private storage and bounded inspection used by import intake.
package ports

import (
	"context"
	"io"
	"time"
)

type ObjectStore interface {
	Put(context.Context, string, string, io.Reader, int64) error
	SignedDownloadURL(context.Context, string, string, time.Duration) (string, error)
}

// Inspector validates a native Word package without external requests or returning source content.
type Inspector interface {
	Inspect(context.Context, io.ReaderAt, int64) error
}
