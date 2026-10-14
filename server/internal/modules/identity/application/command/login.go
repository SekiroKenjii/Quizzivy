package command

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
)

// Login verifies a password and mints a session.
type Login struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
	GeoLabel  string
}

type LoginHandler struct {
	*support.Service
}

// Handle fails with domain.ErrPasswordSignInDisabled, before looking at the email or the password, while the centre has turned password sign-in off.
func (s LoginHandler) Handle(ctx context.Context, cmd Login) (model.Session, error) {
	policy, err := s.SignInPolicy(ctx)
	if err != nil {
		return model.Session{}, fmt.Errorf("read the sign-in policy: %w", err)
	}
	if !policy.Password {
		return model.Session{}, domain.ErrPasswordSignInDisabled
	}
	user, err := s.Users.FindUserByEmail(ctx, cmd.Email)
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		domain.Passwords.BurnTime(ctx, cmd.Password)
		return model.Session{}, domain.ErrInvalidCredentials
	case err != nil:
		return model.Session{}, fmt.Errorf("look up user: %w", err)
	}

	if !user.HasPassword() {
		domain.Passwords.BurnTime(ctx, cmd.Password)
		return model.Session{}, domain.ErrInvalidCredentials
	}

	ok, err := domain.Passwords.Verify(ctx, cmd.Password, *user.PasswordHash)
	if err != nil {
		return model.Session{}, fmt.Errorf("verify password for %s: %w", user.ID, err)
	}
	if !ok {
		return model.Session{}, domain.ErrInvalidCredentials
	}

	if user.Disabled() {
		return model.Session{}, domain.ErrInvalidCredentials
	}

	session, err := s.IssueSession(ctx, user, support.Origin{UserAgent: cmd.UserAgent, IP: cmd.IP, GeoLabel: cmd.GeoLabel}, domain.SignInPassword)
	if errors.Is(err, domain.ErrAccountChanged) {
		return model.Session{}, domain.ErrInvalidCredentials
	}
	return session, err
}
