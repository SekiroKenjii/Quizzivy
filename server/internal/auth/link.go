package auth

import (
	"context"
	"errors"
	"fmt"

	"quizzivy/internal/audit"
	"quizzivy/internal/auth/google"
)

var (
	// ErrLastLoginMethod is §15's rule: unlinking Google from an account with
	// no password would leave no way in at all. The account would still exist,
	// hold its attempts and its enrolments, and be unreachable by its owner.
	ErrLastLoginMethod = errors.New("google is the account's only login method")

	// ErrEmailBelongsToAnotherUser is a Google account whose verified address
	// is already some OTHER Quizzivy account's email.
	//
	// Refused because of where it leads, not where it starts. Linking would
	// bind that Google `sub` to this account, and §5.3's first branch matches
	// on `sub` before email -- so the owner of that address signing in with
	// their own Google would land in someone else's account. Rare, entirely
	// silent, and very hard to diagnose from the inside.
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

	rawIDToken, err := s.google.Exchange(ctx, in.Code, in.CodeVerifier, in.RedirectURI)
	if err != nil {
		return User{}, err
	}
	identity, err := s.google.Verify(ctx, rawIDToken)
	if err != nil {
		return User{}, err
	}
	if !identity.EmailVerified {
		// §5.1 again. An unverified address is not evidence of anything, and
		// binding one to an existing account is the same takeover path sign-in
		// refuses -- with the account already chosen.
		return User{}, google.ErrEmailUnverified
	}

	// Already linked to this very account: a double-submit, or a user who
	// forgot. Nothing to do, and reporting a conflict would be wrong.
	for _, provider := range user.LinkedProviders {
		if provider == "google" {
			existing, err := s.store.FindUserByProviderIdentity(ctx, "google", identity.Subject)
			if err == nil && existing.ID == user.ID {
				return user, nil
			}
			// A different Google account is already linked here. D-08 allows
			// one per user so that "unlink Google" is unambiguous.
			return User{}, ErrIdentityAlreadyLinked
		}
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

	// Re-read: linkedProviders is what the settings screen renders from, and
	// the user we loaded predates the link.
	return s.store.FindUserByID(ctx, user.ID)
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
		// Nothing was linked. The requested state -- no Google on this
		// account -- already holds, so this is a success.
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
