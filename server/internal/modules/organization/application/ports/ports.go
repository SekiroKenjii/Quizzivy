// Package ports is what the organization module needs from outside: an object store for the logos and a checker that decides whether an upload may be one.
package ports

import (
	"context"
	"io"

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
}

// LogoChecker reads an upload and returns it as it may be stored, or refuses
// it with one of the domain's ErrLogo* or ErrSVGNotAllowed reasons.
type LogoChecker interface {
	Check(ctx context.Context, body io.Reader) (domain.LogoFile, error)
}
