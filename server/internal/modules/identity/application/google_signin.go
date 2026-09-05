package application

import (
	"context"
	"errors"
	"fmt"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/google"
)

// GoogleProvider is the pair of calls §5.3 step 3 needs. An interface so the
// resolution order can be tested without reaching Google.
type GoogleProvider interface {
	Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error)
	Verify(ctx context.Context, rawIDToken string) (google.Identity, error)
}

// SelfEnroller creates an account from a join code and enrols it (§6.3).
//
// Defined in terms of internal/join's types rather than a local copy, so
// *join.Service satisfies it directly and there is no adapter to keep in step.
// The dependency runs one way -- join knows nothing about auth -- and §5.3's
// third branch genuinely is "sign-in creates an enrolment", so auth depending
// on enrolment is the real shape rather than a convenience.
type SelfEnroller interface {
	EnrolNewMember(ctx context.Context, m classesdomain.NewMember, rawCode string, meta classesdomain.Meta) (classesdomain.EnrolResult, error)
}

// SetGoogle wires the provider. Nil leaves Google sign-in unavailable rather
// than half-configured.
func (s *Service) SetGoogle(p GoogleProvider, enroller SelfEnroller) {
	s.google = p
	s.enroller = enroller
}

type GoogleSignInInput struct {
	Code         string
	CodeVerifier string
	RedirectURI  string
	JoinCode     string
	UserAgent    string
	IP           string
}

type GoogleSignInResult struct {
	Session       Session
	EnrolledClass *classesdomain.EnrolledClass
}

// GoogleSignIn implements §5.3 in full.
func (s *Service) GoogleSignIn(ctx context.Context, in GoogleSignInInput) (GoogleSignInResult, error) {
	if s.google == nil {
		return GoogleSignInResult{}, domain.ErrGoogleUnavailable
	}
	identity, err := s.verifiedIdentity(ctx, in.Code, in.CodeVerifier, in.RedirectURI)
	if err != nil {
		return GoogleSignInResult{}, err
	}

	// 1. The identity is known.
	user, err := s.users.FindUserByProviderIdentity(ctx, "google", identity.Subject)
	switch {
	case err == nil:
		return s.googleSession(ctx, user, in, nil)
	case !errors.Is(err, domain.ErrUserNotFound):
		return GoogleSignInResult{}, fmt.Errorf("look up google identity: %w", err)
	}

	// 2. A verified email matches an account the teacher already created.
	user, err = s.users.FindUserByEmail(ctx, identity.Email)
	switch {
	case err == nil:
		linked, err := s.linkAndReload(ctx, user.ID, identity)
		if err != nil {
			return GoogleSignInResult{}, err
		}
		return s.googleSession(ctx, linked, in, nil)
	case !errors.Is(err, domain.ErrUserNotFound):
		return GoogleSignInResult{}, fmt.Errorf("look up user by email: %w", err)
	}

	// 3. No match, but a join code: create and enrol (§6.3).
	if in.JoinCode != "" {
		return s.enrolByCode(ctx, identity, in)
	}

	// 4. No match, no join code.
	return GoogleSignInResult{}, domain.ErrAccountNotProvisioned
}

// verifiedIdentity exchanges the code and applies §5.1's one rule: an
// unverified address is refused outright.
func (s *Service) verifiedIdentity(ctx context.Context, code, verifier, redirectURI string) (google.Identity, error) {
	rawIDToken, err := s.google.Exchange(ctx, code, verifier, redirectURI)
	if err != nil {
		return google.Identity{}, err
	}
	identity, err := s.google.Verify(ctx, rawIDToken)
	if err != nil {
		return google.Identity{}, err
	}
	if !identity.EmailVerified {
		return google.Identity{}, google.ErrEmailUnverified
	}
	return identity, nil
}

// linkAndReload attaches the identity to an existing account and reads the
// account back with the link on it.
func (s *Service) linkAndReload(ctx context.Context, userID string, identity google.Identity) (domain.User, error) {
	if err := s.users.LinkIdentity(ctx, userID, "google", identity.Subject, identity.Email); err != nil {
		return domain.User{}, err
	}
	user, err := s.users.FindUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("reload linked user: %w", err)
	}
	return user, nil
}

// enrolByCode is §6.3: a stranger with a valid code becomes a member.
func (s *Service) enrolByCode(ctx context.Context, identity google.Identity, in GoogleSignInInput) (GoogleSignInResult, error) {
	if s.enroller == nil {
		return GoogleSignInResult{}, domain.ErrSelfEnrolNotAvailable
	}
	result, err := s.enroller.EnrolNewMember(ctx,
		classesdomain.NewMember{
			Email:          identity.Email,
			FullName:       identity.Name,
			Provider:       "google",
			ProviderUserID: identity.Subject,
		}, in.JoinCode, classesdomain.Meta{IP: in.IP, UserAgent: in.UserAgent})
	if err != nil {
		return GoogleSignInResult{}, err
	}
	if result.Outcome != classesdomain.PreviewOK {
		return GoogleSignInResult{}, domain.JoinCodeRejected{Outcome: result.Outcome}
	}
	created, err := s.users.FindUserByID(ctx, result.UserID)
	if err != nil {
		return GoogleSignInResult{}, fmt.Errorf("load enrolled member: %w", err)
	}
	return s.googleSession(ctx, created, in, &result.Class)
}

func (s *Service) googleSession(ctx context.Context, user domain.User, in GoogleSignInInput, class *classesdomain.EnrolledClass) (GoogleSignInResult, error) {
	if user.Disabled() {
		return GoogleSignInResult{}, domain.ErrAccountDisabled
	}
	session, err := s.issueSession(ctx, user, in.UserAgent, in.IP)
	if err != nil {
		return GoogleSignInResult{}, err
	}
	return GoogleSignInResult{Session: session, EnrolledClass: class}, nil
}
