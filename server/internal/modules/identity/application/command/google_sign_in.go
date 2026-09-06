package command

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
)

// GoogleSignIn implements §5.3 in full.
type GoogleSignIn struct {
	Code         string
	CodeVerifier string
	RedirectURI  string
	JoinCode     string
	UserAgent    string
	IP           string
}

type GoogleSignInHandler struct {
	*support.Service
}

func (s GoogleSignInHandler) Handle(ctx context.Context, cmd GoogleSignIn) (model.GoogleSignInResult, error) {
	if s.Google == nil {
		return model.GoogleSignInResult{}, domain.ErrGoogleUnavailable
	}
	identity, err := s.VerifiedIdentity(ctx, cmd.Code, cmd.CodeVerifier, cmd.RedirectURI)
	if err != nil {
		return model.GoogleSignInResult{}, err
	}

	user, err := s.Users.FindUserByProviderIdentity(ctx, "google", identity.Subject)
	switch {
	case err == nil:
		return s.GoogleSession(ctx, user, support.GoogleSignInInput(cmd), nil)
	case !errors.Is(err, domain.ErrUserNotFound):
		return model.GoogleSignInResult{}, fmt.Errorf("look up google identity: %w", err)
	}

	user, err = s.Users.FindUserByEmail(ctx, identity.Email)
	switch {
	case err == nil:
		linked, err := s.LinkAndReload(ctx, user.ID, identity)
		if err != nil {
			return model.GoogleSignInResult{}, err
		}
		return s.GoogleSession(ctx, linked, support.GoogleSignInInput(cmd), nil)
	case !errors.Is(err, domain.ErrUserNotFound):
		return model.GoogleSignInResult{}, fmt.Errorf("look up user by email: %w", err)
	}

	if cmd.JoinCode != "" {
		return s.EnrolByCode(ctx, identity, support.GoogleSignInInput(cmd))
	}

	return model.GoogleSignInResult{}, domain.ErrAccountNotProvisioned
}
