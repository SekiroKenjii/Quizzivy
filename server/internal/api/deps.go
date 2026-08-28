package api

import (
	"context"
	"errors"

	"quizzivy/internal/auth"
	"quizzivy/internal/httpx"
	"quizzivy/internal/join"
)

// DB is the slice of the pool handlers need. An interface rather than the
// concrete pool so tests can substitute one without a live database.
type DB interface {
	Ping(ctx context.Context) error
}

// AuthService is the slice of internal/auth the handlers use. An interface so
// a handler test can supply a fake without a database.
type AuthService interface {
	Login(ctx context.Context, in auth.LoginInput) (auth.Session, error)
	Refresh(ctx context.Context, in auth.RefreshInput) (auth.RefreshResult, error)
	Logout(ctx context.Context, token string) error
	CurrentUser(ctx context.Context, userID string) (auth.User, error)
	ChangePassword(ctx context.Context, in auth.ChangePasswordInput) error
	GoogleSignIn(ctx context.Context, in auth.GoogleSignInInput) (auth.GoogleSignInResult, error)
	LinkGoogle(ctx context.Context, in auth.LinkGoogleInput) (auth.User, error)
	UnlinkGoogle(ctx context.Context, userID, ip, userAgent string) error
}

// JoinService is the slice of internal/join the handlers use.
type JoinService interface {
	Rotate(ctx context.Context, req join.RotateRequest) (join.Rotated, error)
	Revoke(ctx context.Context, req join.RevokeRequest) error
	Preview(ctx context.Context, rawCode string) (join.PreviewResult, error)
	EnrolExisting(ctx context.Context, userID, rawCode string, meta join.Meta) (join.EnrolResult, error)
}

// TokenVerifier checks an access token. Separate from AuthService because the
// auth middleware needs it before any handler runs, and because verification is
// pure -- no database, no state.
type TokenVerifier interface {
	Verify(raw string) (*auth.Claims, error)
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
