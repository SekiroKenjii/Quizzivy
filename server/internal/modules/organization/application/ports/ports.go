// Package ports is what the organization module needs from outside: an object store for the logos, a checker that decides whether an upload may be one, and the mail dispatcher to wake when the usage limits change.
package ports

import (
	"context"
	"io"
	"time"

	"quizzivy/internal/modules/organization/domain"
)

// LogoStore keeps logos, content-addressed under the keys the module names.
type LogoStore interface {
	// Put stores body under key unless the same bytes are there already; digest is body's SHA-256.
	Put(ctx context.Context, key string, kind domain.LogoType, body []byte, digest []byte) error
	// Get returns the bytes under key, or domain.ErrNoLogo when there are none.
	Get(ctx context.Context, key string) ([]byte, error)
	// Exists reports whether key holds bytes.
	Exists(ctx context.Context, key string) (bool, error)
	// List returns at most limit objects whose keys begin with prefix, in key order.
	List(ctx context.Context, prefix string, limit int) ([]StoredLogo, error)
	// Delete removes the object under key. A key with no object is not an error.
	Delete(ctx context.Context, key string) error
}

// StoredLogo is an object under the logo prefix and when it was last written.
type StoredLogo struct {
	Key       string
	WrittenAt time.Time
}

// LogoChecker reads an upload and returns it as it may be stored, or refuses
// it with one of the domain's ErrLogo* reasons.
type LogoChecker interface {
	Check(ctx context.Context, body io.Reader) (domain.LogoFile, error)
}

// MailWake wakes the mail dispatcher, so that mail a sending cap held goes out
// as soon as a save raises the cap rather than when the dispatcher next wakes.
type MailWake interface {
	Notify()
}
