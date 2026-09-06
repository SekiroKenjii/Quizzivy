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
}

type LoginHandler struct {
	*support.Service
}

func (s LoginHandler) Handle(ctx context.Context, cmd Login) (model.Session, error) {
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

	return s.IssueSession(ctx, user, cmd.UserAgent, cmd.IP)
}
