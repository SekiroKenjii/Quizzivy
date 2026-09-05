package core

import (
	"context"
	"errors"
	identityapp "quizzivy/internal/modules/identity/application"

	"quizzivy/internal/platform/httpx"
)

// DB is the slice of the pool handlers need. An interface rather than the
// concrete pool so tests can substitute one without a live database.
type DB interface {
	Ping(ctx context.Context) error
}

// TokenVerifier checks an access token. Separate from AuthService because the
// auth middleware needs it before any handler runs, and because verification is
// pure -- no database, no state.
type TokenVerifier interface {
	Verify(raw string) (*identityapp.Claims, error)
}

// verifyAccessToken adapts the token issuer to what the middleware wants.
//
// A nil verifier is a wiring mistake, not a caller error: refusing every
// request is the only safe response, and it is loud enough to find in one run.
func (d Deps) verifyAccessToken(bearer string) (httpx.Principal, error) {
	if d.Tokens == nil {
		return httpx.Principal{}, errors.New("no token verifier configured")
	}
	claims, err := d.Tokens.Verify(bearer)
	if err != nil {
		return httpx.Principal{}, err
	}
	return httpx.Principal{UserID: claims.Subject, Role: claims.Role}, nil
}
