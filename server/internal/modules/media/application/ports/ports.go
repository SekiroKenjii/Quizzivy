package ports

import (
	"context"
	"io"
	"time"
)

type

// ObjectStore is the slice of internal/storage this package uses.
ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type

// AudioProbe measures an upload that is not an image: its MIME type and its
// duration, failing with the domain's ErrUnsupportedType or ErrUnmeasurable.
AudioProbe interface {
	Audio(r io.ReaderAt, size int64) (mime string, durationMs int, err error)
}
