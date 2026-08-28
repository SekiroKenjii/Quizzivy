package auth

import (
	"context"
	"errors"
	"fmt"

	"quizzivy/internal/auth/google"
	"quizzivy/internal/join"
)

var (
	// ErrAccountNotProvisioned is §5.3 step 4's last branch: a real Google
	// identity that matches nothing here and arrived without a join code.
	//
	// Deliberately explicit rather than opaque. The person has just proved to
	// Google that they control this address, so telling them "you need a class
	// code" reveals nothing they could not confirm themselves -- and the
	// alternative is a sign-in that fails with no way forward.
	ErrAccountNotProvisioned = errors.New("account not provisioned")

	// ErrGoogleUnavailable means Google sign-in is not configured on this
	// deployment. Distinct from a failed sign-in: nothing the user does will fix
	// it, so it must not be reported as their problem.
	ErrGoogleUnavailable = errors.New("google sign-in is not configured")

	// ErrSelfEnrolNotAvailable marks the branch this task does not own. Creating
	// and enrolling an account from a join code is T-1.8; until it lands, a
	// joinCode reaches a seam rather than a wrong answer.
	ErrSelfEnrolNotAvailable = errors.New("join-code signup is not implemented yet")
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
	EnrolNewMember(ctx context.Context, m join.NewMember, rawCode string, meta join.Meta) (join.EnrolResult, error)
}

// JoinCodeRejected is a join code that did not pass. It carries the outcome so
// the HTTP layer can reuse /join/preview's exact mapping: the same four codes,
// the same leak rules, decided in one place rather than two that drift.
type JoinCodeRejected struct {
	Outcome join.PreviewOutcome
}

func (e JoinCodeRejected) Error() string {
	return fmt.Sprintf("join code rejected (outcome %d)", e.Outcome)
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
	EnrolledClass *join.EnrolledClass
}

// GoogleSignIn implements §5.3 in full.
//
// The order of the branches is the security property. Identity before email
// means a Google account that has signed in before is matched on its immutable
// `sub`, not on an address that may since have moved to someone else. Email
// only ever matches when it is VERIFIED, which is checked before any lookup
// happens -- an unverified address is not evidence of anything, and treating it
// as a match would let anyone who can register an address with Google claim the
// matching Quizzivy account (§5.1).
func (s *Service) GoogleSignIn(ctx context.Context, in GoogleSignInInput) (GoogleSignInResult, error) {
	if s.google == nil {
		return GoogleSignInResult{}, ErrGoogleUnavailable
	}

	rawIDToken, err := s.google.Exchange(ctx, in.Code, in.CodeVerifier, in.RedirectURI)
	if err != nil {
		return GoogleSignInResult{}, err
	}
	identity, err := s.google.Verify(ctx, rawIDToken)
	if err != nil {
		return GoogleSignInResult{}, err
	}

	// Before ANY lookup. An unverified email must not be able to reach the
	// matching branch below, even to be rejected there -- and it must not be
	// rescued by a join code either.
	if !identity.EmailVerified {
		return GoogleSignInResult{}, google.ErrEmailUnverified
	}

	// 1. The identity is known.
	user, err := s.store.FindUserByProviderIdentity(ctx, "google", identity.Subject)
	switch {
	case err == nil:
		return s.googleSession(ctx, user, in, nil)
	case !errors.Is(err, ErrUserNotFound):
		return GoogleSignInResult{}, fmt.Errorf("look up google identity: %w", err)
	}

	// 2. A verified email matches an account the teacher already created.
	user, err = s.store.FindUserByEmail(ctx, identity.Email)
	switch {
	case err == nil:
		// Link before checking `disabled`, so a suspended account does not
		// quietly accumulate a second unlinked Google identity on every attempt.
		if err := s.store.LinkIdentity(ctx, user.ID, "google", identity.Subject, identity.Email); err != nil {
			return GoogleSignInResult{}, err
		}
		// Re-read: the user we loaded predates the link, so its
		// linkedProviders is already stale.
		user, err = s.store.FindUserByID(ctx, user.ID)
		if err != nil {
			return GoogleSignInResult{}, fmt.Errorf("reload linked user: %w", err)
		}
		return s.googleSession(ctx, user, in, nil)
	case !errors.Is(err, ErrUserNotFound):
		return GoogleSignInResult{}, fmt.Errorf("look up user by email: %w", err)
	}

	// 3. No match, but a join code: create and enrol (§6.3).
	if in.JoinCode != "" {
		if s.enroller == nil {
			return GoogleSignInResult{}, ErrSelfEnrolNotAvailable
		}
		result, err := s.enroller.EnrolNewMember(ctx,
			join.NewMember{
				Email:          identity.Email,
				FullName:       identity.Name,
				Provider:       "google",
				ProviderUserID: identity.Subject,
			}, in.JoinCode, join.Meta{IP: in.IP, UserAgent: in.UserAgent})
		if err != nil {
			return GoogleSignInResult{}, err
		}
		if result.Outcome != join.PreviewOK {
			// The account is NOT created: Enrol validates the code before it
			// creates anything, and the whole thing is one transaction.
			return GoogleSignInResult{}, JoinCodeRejected{Outcome: result.Outcome}
		}

		created, err := s.store.FindUserByID(ctx, result.UserID)
		if err != nil {
			return GoogleSignInResult{}, fmt.Errorf("load enrolled member: %w", err)
		}
		return s.googleSession(ctx, created, in, &result.Class)
	}

	// 4. No match, no join code.
	return GoogleSignInResult{}, ErrAccountNotProvisioned
}

func (s *Service) googleSession(ctx context.Context, user User, in GoogleSignInInput, class *join.EnrolledClass) (GoogleSignInResult, error) {
	// §5.1: a suspended account cannot sign in, by any route. Checked here
	// rather than in each branch so no branch can forget.
	if user.Disabled() {
		return GoogleSignInResult{}, ErrAccountDisabled
	}
	session, err := s.issueSession(ctx, user, in.UserAgent, in.IP)
	if err != nil {
		return GoogleSignInResult{}, err
	}
	return GoogleSignInResult{Session: session, EnrolledClass: class}, nil
}
