package command

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"slices"
)

// LinkGoogle attaches a Google identity to the signed-in account (§15).
type LinkGoogle struct {
	UserID       string
	Code         string
	CodeVerifier string
	RedirectURI  string
	IP           string
	UserAgent    string
}

type LinkGoogleHandler struct {
	*support.Service
}

func (s LinkGoogleHandler) Handle(ctx context.Context, cmd LinkGoogle) (domain.User, error) {
	if s.Google == nil {
		return domain.User{}, domain.ErrGoogleUnavailable
	}

	user, err := s.Users.FindUserByID(ctx, cmd.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if user.Disabled() {
		return domain.User{}, domain.ErrAccountDisabled
	}

	identity, err := s.VerifiedIdentity(ctx, cmd.Code, cmd.CodeVerifier, cmd.RedirectURI)
	if err != nil {
		return domain.User{}, err
	}
	if slices.Contains(user.LinkedProviders, "google") {
		return s.AlreadyLinked(ctx, user, identity)
	}

	owner, err := s.Users.FindUserByEmail(ctx, identity.Email)
	switch {
	case err == nil && owner.ID != user.ID:
		return domain.User{}, domain.ErrEmailBelongsToAnotherUser
	case err != nil && !errors.Is(err, domain.ErrUserNotFound):
		return domain.User{}, fmt.Errorf("check google address ownership: %w", err)
	}

	if err := s.Users.LinkIdentity(ctx, user.ID, "google", identity.Subject, identity.Email); err != nil {
		return domain.User{}, err
	}
	if err := s.Users.WriteAudit(ctx, audit.Entry{
		ActorUserID: &user.ID,
		Action:      "user.google_linked",
		Entity:      "user_identity",
		EntityID:    &user.ID,
		OccurredAt:  s.Now(),
		IP:          opt.String(cmd.IP),
		UserAgent:   opt.String(cmd.UserAgent),
	}); err != nil {
		return domain.User{}, err
	}
	return s.Users.FindUserByID(ctx, user.ID)
}
