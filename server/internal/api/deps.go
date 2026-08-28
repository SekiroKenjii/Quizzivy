package api

import (
	"context"

	"quizzivy/internal/auth"
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
}
