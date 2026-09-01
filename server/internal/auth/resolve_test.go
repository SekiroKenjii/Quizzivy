package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/auth"
	"quizzivy/internal/auth/google"
	"quizzivy/internal/join"
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
	providers := result.Session.User.LinkedProviders
	if len(providers) != 1 || providers[0] != "google" {
		t.Errorf("linkedProviders = %v, want [google]", providers)
	}

	// And signing in again takes branch 1.
	if _, err := signIn(svc, ""); err != nil {
		t.Errorf("second sign-in failed: %v", err)
	}
}

func TestBranch3AJoinCodeCreatesAndEnrols(t *testing.T) {
	// §5.3's third branch, and the only self-signup path in the product.
	pool := newPool(t)
	classID, teacherID := makeClassForEnrol(t, pool)
	svc, _ := googleServiceWithEnroller(t, pool, verifiedIdentity("brand-new@example.com"))
	code := issueJoinCode(t, pool, classID, teacherID)

	result, err := signIn(svc, code)
	if err != nil {
		t.Fatalf("sign-in with a join code: %v", err)
	}
	if result.EnrolledClass == nil {
		t.Fatal("no enrolledClass on a signup that enrolled")
	}
	if result.EnrolledClass.ID != classID {
		t.Errorf("enrolled in %s, want %s", result.EnrolledClass.ID, classID)
	}
	if result.Session.AccessToken == "" {
		t.Error("no session was issued to the new member")
	}
	if result.Session.User.Role != "student" {
		t.Errorf("role = %s, want student", result.Session.User.Role)
	}
	// Google-only, per §6.3: there is no password to set.
	if result.Session.User.HasPassword() {
		t.Error("a self-join account was created with a password")
	}
	t.Cleanup(func() { deleteUser(t, pool, result.Session.User.ID) })
}

func TestBranch3RejectsABadJoinCodeWithoutCreatingAnyone(t *testing.T) {
	pool := newPool(t)
	svc, _ := googleServiceWithEnroller(t, pool, verifiedIdentity("never-created@example.com"))

	var rejected auth.JoinCodeRejected
	if _, err := signIn(svc, "ZZZZ-ZZZZ"); !errors.As(err, &rejected) {
		t.Fatalf("error = %v, want auth.JoinCodeRejected", err)
	}

	var users int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.users WHERE email = $1`, "never-created@example.com").Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Error("a rejected join code still created an account")
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
	pool := newPool(t)
	svc := newService(t, pool)

	if _, err := signIn(svc, ""); !errors.Is(err, auth.ErrGoogleUnavailable) {
		t.Fatalf("error = %v, want ErrGoogleUnavailable", err)
	}
}

// The §5.3 branch-3 fixtures. These reach into internal/join because branch 3
// IS an enrolment -- testing it against a fake enroller would assert only that
// the wiring compiles.

func googleServiceWithEnroller(t *testing.T, pool *pgxpool.Pool, identity google.Identity) (*auth.Service, *stubGoogle) {
	t.Helper()
	svc := newService(t, pool)
	stub := &stubGoogle{identity: identity}
	svc.SetGoogle(stub, join.NewService(join.NewStore(pool)))
	return svc, stub
}

func makeClassForEnrol(t *testing.T, pool *pgxpool.Pool) (classID, teacherID string) {
	t.Helper()
	ctx := context.Background()
	teacherID, _ = makeUser(t, pool, admin)
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.classes (name) VALUES ('Lớp ghi danh') RETURNING id::text`).Scan(&classID); err != nil {
		t.Fatalf("insert class: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_join_codes WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1`, classID)
	})
	return classID, teacherID
}

func issueJoinCode(t *testing.T, pool *pgxpool.Pool, classID, teacherID string) string {
	t.Helper()
	rotated, err := join.NewService(join.NewStore(pool)).Rotate(context.Background(),
		join.RotateRequest{ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatalf("issue join code: %v", err)
	}
	return rotated.Code
}

// deleteUser removes an account the test caused to be created, which makeUser's
// cleanup cannot know about.
func deleteUser(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	c := context.Background()
	_, _ = pool.Exec(c, `DELETE FROM app.refresh_tokens WHERE user_id = $1`, userID)
	_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id = $1`, userID)
	_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE user_id = $1`, userID)
	_, _ = pool.Exec(c, `DELETE FROM app.user_identities WHERE user_id = $1`, userID)
	_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1`, userID)
}
