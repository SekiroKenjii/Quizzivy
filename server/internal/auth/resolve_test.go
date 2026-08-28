package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/auth"
	"quizzivy/internal/auth/google"
)

// §5.3 step 4's resolution order, one test per branch. The order is the
// security property, not an implementation detail: identity before email means
// a returning Google account is matched on its immutable `sub` rather than on
// an address that may since have moved to someone else.

// stubGoogle stands in for the exchange and the token verification, so these
// tests exercise the RESOLUTION and nothing else.
type stubGoogle struct {
	identity  google.Identity
	exchange  error
	verify    error
	exchanged bool
}

func (s *stubGoogle) Exchange(context.Context, string, string, string) (string, error) {
	s.exchanged = true
	return "an.id.token", s.exchange
}

func (s *stubGoogle) Verify(context.Context, string) (google.Identity, error) {
	return s.identity, s.verify
}

func googleService(t *testing.T, pool *pgxpool.Pool, identity google.Identity) (*auth.Service, *stubGoogle) {
	t.Helper()
	svc := newService(t, pool)
	stub := &stubGoogle{identity: identity}
	svc.SetGoogle(stub, nil)
	return svc, stub
}

func verifiedIdentity(email string) google.Identity {
	return google.Identity{
		Subject:       "google-sub-" + email,
		Email:         email,
		EmailVerified: true,
		Name:          "Nguyễn Văn A",
	}
}

func signIn(svc *auth.Service, joinCode string) (auth.GoogleSignInResult, error) {
	return svc.GoogleSignIn(context.Background(), auth.GoogleSignInInput{
		Code: "c", CodeVerifier: "v", RedirectURI: "https://app.quizzivy.com/cb",
		JoinCode: joinCode, IP: "203.0.113.11", UserAgent: "go-test",
	})
}

func TestBranch1AKnownIdentitySignsIn(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool, googleOnly)
	identity := verifiedIdentity(email)
	linkGoogleSubject(t, pool, id, identity.Subject, email)

	svc, _ := googleService(t, pool, identity)
	result, err := signIn(svc, "")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if result.Session.User.ID != id {
		t.Errorf("signed in as %s, want %s", result.Session.User.ID, id)
	}
	if result.Session.AccessToken == "" || result.Session.RefreshToken == "" {
		t.Error("no session was issued")
	}
	if result.EnrolledClass != nil {
		t.Error("enrolledClass set for a plain sign-in")
	}
}

func TestBranch1MatchesOnSubjectNotEmail(t *testing.T) {
	// A Google account's email can change; its `sub` cannot. Matching on the
	// subject is what keeps a returning user attached to their own account
	// after they change their Gmail address -- and, more importantly, what
	// stops a NEW owner of a recycled address from inheriting one.
	pool := newPool(t)
	id, email := makeUser(t, pool, googleOnly)
	linkGoogleSubject(t, pool, id, "the-stable-subject", email)

	identity := google.Identity{
		Subject:       "the-stable-subject",
		Email:         "changed-address@example.com", // no user has this
		EmailVerified: true,
	}
	svc, _ := googleService(t, pool, identity)

	result, err := signIn(svc, "")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if result.Session.User.ID != id {
		t.Errorf("signed in as %s, want %s", result.Session.User.ID, id)
	}
}

func TestBranch2AVerifiedEmailLinksToAnExistingAccount(t *testing.T) {
	// The teacher creates the account; the student signs in with Google for the
	// first time. This is the only path that ever creates a link.
	pool := newPool(t)
	id, email := makeUser(t, pool)
	identity := verifiedIdentity(email)

	svc, _ := googleService(t, pool, identity)
	result, err := signIn(svc, "")
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if result.Session.User.ID != id {
		t.Fatalf("signed in as %s, want %s", result.Session.User.ID, id)
	}

	// The response must reflect the link that was just made, not the state
	// before it -- the settings screen reads linkedProviders from here.
	providers := result.Session.User.LinkedProviders
	if len(providers) != 1 || providers[0] != "google" {
		t.Errorf("linkedProviders = %v, want [google]", providers)
	}

	// And signing in again takes branch 1.
	if _, err := signIn(svc, ""); err != nil {
		t.Errorf("second sign-in failed: %v", err)
	}
}

func TestBranch3AJoinCodeReachesTheEnrolmentSeam(t *testing.T) {
	// Creating and enrolling an account is T-1.8. Until it lands, a join code
	// must reach a seam that says so, rather than falling through to
	// ACCOUNT_NOT_PROVISIONED -- which would be a wrong answer, not a missing one.
	pool := newPool(t)
	svc, _ := googleService(t, pool, verifiedIdentity("brand-new@example.com"))

	_, err := signIn(svc, "ABCD2345")
	if !errors.Is(err, auth.ErrSelfEnrolNotAvailable) {
		t.Fatalf("error = %v, want ErrSelfEnrolNotAvailable", err)
	}
}

func TestBranch4NoMatchAndNoJoinCodeIsNotProvisioned(t *testing.T) {
	pool := newPool(t)
	svc, _ := googleService(t, pool, verifiedIdentity("stranger@example.com"))

	if _, err := signIn(svc, ""); !errors.Is(err, auth.ErrAccountNotProvisioned) {
		t.Fatalf("error = %v, want ErrAccountNotProvisioned", err)
	}
}

func TestAnUnverifiedEmailIsRejectedEvenWhenItMatchesAUser(t *testing.T) {
	// §5.1, non-negotiable. Anyone who can get Google to issue a token for an
	// address they have not proved they own could otherwise claim the matching
	// Quizzivy account. The check runs BEFORE any lookup, so the matching
	// branch is never even reached.
	pool := newPool(t)
	id, email := makeUser(t, pool)

	identity := verifiedIdentity(email)
	identity.EmailVerified = false
	svc, _ := googleService(t, pool, identity)

	if _, err := signIn(svc, ""); !errors.Is(err, google.ErrEmailUnverified) {
		t.Fatalf("error = %v, want ErrEmailUnverified", err)
	}

	// No link was created on the way out.
	var links int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.user_identities WHERE user_id = $1`, id).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Errorf("%d identities linked for an unverified email", links)
	}
}

func TestAnUnverifiedEmailIsNotRescuedByAJoinCode(t *testing.T) {
	pool := newPool(t)
	identity := verifiedIdentity("new-person@example.com")
	identity.EmailVerified = false
	svc, _ := googleService(t, pool, identity)

	if _, err := signIn(svc, "ABCD2345"); !errors.Is(err, google.ErrEmailUnverified) {
		t.Fatalf("error = %v, want ErrEmailUnverified", err)
	}
}

func TestASuspendedAccountCannotSignInWithGoogle(t *testing.T) {
	// §5.1 applies to every route in, not just the password one.
	pool := newPool(t)
	id, email := makeUser(t, pool, disabled)
	identity := verifiedIdentity(email)
	linkGoogleSubject(t, pool, id, identity.Subject, email)

	svc, _ := googleService(t, pool, identity)
	if _, err := signIn(svc, ""); !errors.Is(err, auth.ErrAccountDisabled) {
		t.Fatalf("error = %v, want ErrAccountDisabled", err)
	}
}

func TestAnAccountLinkedToADifferentGoogleAccountIsRefused(t *testing.T) {
	// D-08 allows one Google identity per user, so that "unlink Google" is
	// unambiguous. A second one is refused rather than silently added.
	pool := newPool(t)
	id, email := makeUser(t, pool)
	linkGoogleSubject(t, pool, id, "the-first-google-account", email)

	// Same email, different Google account.
	identity := google.Identity{Subject: "a-second-google-account", Email: email, EmailVerified: true}
	svc, _ := googleService(t, pool, identity)

	if _, err := signIn(svc, ""); !errors.Is(err, auth.ErrIdentityAlreadyLinked) {
		t.Fatalf("error = %v, want ErrIdentityAlreadyLinked", err)
	}
}

func TestAFailedExchangeNeverReachesTheDatabase(t *testing.T) {
	pool := newPool(t)
	svc, stub := googleService(t, pool, verifiedIdentity("whoever@example.com"))
	stub.exchange = google.ErrExchangeFailed

	if _, err := signIn(svc, ""); !errors.Is(err, google.ErrExchangeFailed) {
		t.Fatalf("error = %v, want ErrExchangeFailed", err)
	}
}

func TestGoogleSignInIsUnavailableWhenUnconfigured(t *testing.T) {
	// A deployment may legitimately run on password login alone. That is not a
	// failed sign-in and must not be reported as one.
	pool := newPool(t)
	svc := newService(t, pool)

	if _, err := signIn(svc, ""); !errors.Is(err, auth.ErrGoogleUnavailable) {
		t.Fatalf("error = %v, want ErrGoogleUnavailable", err)
	}
}
