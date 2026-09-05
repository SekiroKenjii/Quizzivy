package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"quizzivy/internal/audit"
	"quizzivy/internal/auth/google"
)

var (
	ErrLastLoginMethod           = errors.New("google is the account's only login method")
	ErrEmailBelongsToAnotherUser = errors.New("that Google address belongs to another account")
)

type LinkGoogleInput struct {
	UserID       string
	Code         string
	CodeVerifier string
	RedirectURI  string
	IP           string
	UserAgent    string
}

// LinkGoogle attaches a Google identity to the signed-in account (§15).
//
// The email does NOT have to match the account's own: a teacher linking a
// personal Gmail to a work address is the ordinary case. What is checked is
// that the address is verified -- the same §5.1 rule sign-in applies, for the
// same reason -- and that it is not already some other account's email.
func (s *Service) LinkGoogle(ctx context.Context, in LinkGoogleInput) (User, error) {
	if s.google == nil {
		return User{}, ErrGoogleUnavailable
	}

	user, err := s.store.FindUserByID(ctx, in.UserID)
	if err != nil {
		return User{}, err
	}
	if user.Disabled() {
		return User{}, ErrAccountDisabled
	}

	identity, err := s.verifiedIdentity(ctx, in.Code, in.CodeVerifier, in.RedirectURI)
	if err != nil {
		return User{}, err
	}
	if slices.Contains(user.LinkedProviders, "google") {
		return s.alreadyLinked(ctx, user, identity)
	}

	owner, err := s.store.FindUserByEmail(ctx, identity.Email)
	switch {
	case err == nil && owner.ID != user.ID:
		return User{}, ErrEmailBelongsToAnotherUser
	case err != nil && !errors.Is(err, ErrUserNotFound):
		return User{}, fmt.Errorf("check google address ownership: %w", err)
	}

	if err := s.store.LinkIdentity(ctx, user.ID, "google", identity.Subject, identity.Email); err != nil {
		return User{}, err
	}
	if err := s.store.WriteAudit(ctx, audit.Entry{
		ActorUserID: &user.ID,
		Action:      "user.google_linked",
		Entity:      "user_identity",
		EntityID:    &user.ID,
		OccurredAt:  s.now(),
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return User{}, err
	}
	return s.store.FindUserByID(ctx, user.ID)
}

// alreadyLinked answers a second link: the same Google account is a no-op,
// a different one is refused.
func (s *Service) alreadyLinked(ctx context.Context, user User, identity google.Identity) (User, error) {
	existing, err := s.store.FindUserByProviderIdentity(ctx, "google", identity.Subject)
	if err == nil && existing.ID == user.ID {
		return user, nil
	}
	return User{}, ErrIdentityAlreadyLinked
}

// UnlinkGoogle detaches the Google identity (§15).
//
// Refused when the account has no password, because the result would be an
// account nobody can sign into -- still holding its attempts and enrolments,
// and unreachable by its owner. The client disables the control and explains
// why; this is the server making sure that explanation is true.
func (s *Service) UnlinkGoogle(ctx context.Context, userID, ip, userAgent string) error {
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Disabled() {
		return ErrAccountDisabled
	}
	if !user.HasPassword() {
		return ErrLastLoginMethod
	}

	removed, err := s.store.UnlinkIdentity(ctx, userID, "google")
	if err != nil {
		return err
	}
	if !removed {
		return nil
	}

	return s.store.WriteAudit(ctx, audit.Entry{
		ActorUserID: &userID,
		Action:      "user.google_unlinked",
		Entity:      "user_identity",
		EntityID:    &userID,
		OccurredAt:  s.now(),
		IP:          optional(ip),
		UserAgent:   optional(userAgent),
	})
}
