package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/auth"
)

// Login is the one public, unauthenticated endpoint that touches credentials.
// These run against a real database because the interesting behaviour -- what
// a disabled account does, what a Google-only account does -- lives in the
// interaction between the query and the check order, not in either alone.

const testPassword = "mật-khẩu-đúng"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping login integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return pool
}

func newService(t *testing.T, pool *pgxpool.Pool) *auth.Service {
	t.Helper()
	issuer, err := auth.NewTokenIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return auth.NewService(auth.NewStore(pool), issuer, 30*24*time.Hour)
}

// makeUser inserts a user with a generated email so tests never collide with
// each other or with seed data, and removes it afterwards.
func makeUser(t *testing.T, pool *pgxpool.Pool, opts ...func(*userSpec)) (id, email string) {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	spec := userSpec{
		email:    "login-" + hex.EncodeToString(nonce) + "@example.com",
		fullName: "Nguyễn Văn A",
		role:     "student",
	}
	hash, err := auth.HashPassword(context.Background(), testPassword)
	if err != nil {
		t.Fatal(err)
	}
	spec.passwordHash = &hash
	for _, o := range opts {
		o(&spec)
	}

	ctx := context.Background()
	err = pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role, password_hash, disabled_at)
		 VALUES ($1, $2, $3::app.user_role, $4, $5) RETURNING id::text`,
		spec.email, spec.fullName, spec.role, spec.passwordHash, spec.disabledAt).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM app.refresh_tokens WHERE user_id = $1`, id)
		// audit_log survives its actor by design (ON DELETE SET NULL), so a
		// test that triggers an audit write has to clear it explicitly or it
		// accumulates orphan rows in the development database.
		_, _ = pool.Exec(ctx, `DELETE FROM app.audit_log WHERE actor_user_id = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM app.users WHERE id = $1`, id)
	})
	return id, spec.email
}

type userSpec struct {
	email        string
	fullName     string
	role         string
	passwordHash *string
	disabledAt   *time.Time
}

func googleOnly(s *userSpec) { s.passwordHash = nil }
func admin(s *userSpec)      { s.role = "admin" }
func disabled(s *userSpec)   { now := time.Now(); s.disabledAt = &now }

func TestCorrectPasswordSucceeds(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool, admin)

	session, err := svc.Login(context.Background(), auth.LoginInput{
		Email: email, Password: testPassword, IP: "203.0.113.5", UserAgent: "test",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if session.User.ID != id {
		t.Errorf("user id = %s, want %s", session.User.ID, id)
	}
	if session.User.Role != "admin" {
		t.Errorf("role = %s, want admin", session.User.Role)
	}
	if session.AccessToken == "" || session.RefreshToken == "" {
		t.Error("session is missing a token")
	}
	if session.ExpiresIn != 900 {
		t.Errorf("expiresIn = %d, want 900 (§5.2's ~15 minutes)", session.ExpiresIn)
	}
}

func TestOnlyTheHashOfTheRefreshTokenIsStored(t *testing.T) {
	// §13.5. A database dump must not hand over live sessions.
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)

	session, err := svc.Login(context.Background(), auth.LoginInput{Email: email, Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}

	var stored []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT token_hash FROM app.refresh_tokens WHERE user_id = $1`, id).Scan(&stored); err != nil {
		t.Fatalf("no refresh token row was written: %v", err)
	}
	want := sha256.Sum256([]byte(session.RefreshToken))
	if string(stored) != string(want[:]) {
		t.Error("stored value is not the SHA-256 of the issued token")
	}

	var plaintextRows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.refresh_tokens WHERE encode(token_hash, 'escape') = $1`,
		session.RefreshToken).Scan(&plaintextRows); err != nil {
		t.Fatal(err)
	}
	if plaintextRows != 0 {
		t.Error("the plaintext refresh token appears in the database")
	}
}

// Every failure path returns the SAME error. §6.5 asks the join endpoints not
// to reveal which classes exist; the same reasoning applies to which accounts
// exist, and to which are suspended -- "this account is disabled" confirms the
// email is real and worth attacking elsewhere.
func TestEveryFailureLooksIdentical(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	_, goodEmail := makeUser(t, pool)
	_, googleEmail := makeUser(t, pool, googleOnly)
	_, disabledEmail := makeUser(t, pool, disabled)

	cases := map[string]auth.LoginInput{
		"no such user":             {Email: "nobody-here@example.com", Password: testPassword},
		"wrong password":           {Email: goodEmail, Password: "sai-mật-khẩu"},
		"google-only account":      {Email: googleEmail, Password: testPassword},
		"disabled, right password": {Email: disabledEmail, Password: testPassword},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Login(ctx, in)
			if !errors.Is(err, auth.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestDisabledAccountCannotLogInEvenWithTheRightPassword(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool, disabled)

	if _, err := svc.Login(context.Background(), auth.LoginInput{Email: email, Password: testPassword}); err == nil {
		t.Fatal("a disabled account logged in")
	}

	// And no session was minted on the way to refusing.
	var tokens int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1`, id).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 0 {
		t.Errorf("%d refresh tokens were created for a disabled account", tokens)
	}
}

func TestEmailMatchIsCaseInsensitive(t *testing.T) {
	// The lookup must use the same lower(email) expression as the unique index,
	// or a user who types their email with different capitalisation than they
	// registered with cannot log in at all.
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)

	if _, err := svc.Login(context.Background(), auth.LoginInput{
		Email: strings.ToUpper(email), Password: testPassword,
	}); err != nil {
		t.Errorf("upper-cased email failed to log in: %v", err)
	}
}

func TestDisabledCostsTheSameTimeAsAWrongPassword(t *testing.T) {
	// The check order is what makes this true: the password is verified BEFORE
	// disabled is examined. Checking disabled first would return in
	// microseconds and make suspended accounts detectable by timing alone.
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	_, activeEmail := makeUser(t, pool)
	_, disabledEmail := makeUser(t, pool, disabled)

	median := func(in auth.LoginInput) time.Duration {
		const runs = 5
		var samples []time.Duration
		for i := 0; i < runs; i++ {
			start := time.Now()
			_, _ = svc.Login(ctx, in)
			samples = append(samples, time.Since(start))
		}
		for i := 1; i < len(samples); i++ {
			for j := i; j > 0 && samples[j] < samples[j-1]; j-- {
				samples[j], samples[j-1] = samples[j-1], samples[j]
			}
		}
		return samples[runs/2]
	}

	wrong := median(auth.LoginInput{Email: activeEmail, Password: "sai"})
	disabledTime := median(auth.LoginInput{Email: disabledEmail, Password: testPassword})

	ratio := float64(disabledTime) / float64(wrong)
	if ratio < 0.5 || ratio > 2.0 {
		t.Errorf("disabled %v vs wrong-password %v (ratio %.2f): a suspended account is "+
			"distinguishable by timing", disabledTime, wrong, ratio)
	}
}
