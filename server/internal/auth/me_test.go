package auth_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/auth"
)

// linkGoogle attaches a Google identity, which is what makes linkedProviders
// non-empty. makeUser's `googleOnly` only clears the password; without an
// identity row the account could not sign in at all.
func linkGoogle(t *testing.T, pool *pgxpool.Pool, userID, email string) {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	linkGoogleSubject(t, pool, userID, hex.EncodeToString(nonce), email)
}

// linkGoogleSubject links a CHOSEN Google subject, which the §5.3 resolution
// tests need in order to control which branch a sign-in takes.
func linkGoogleSubject(t *testing.T, pool *pgxpool.Pool, userID, subject, email string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
		 VALUES ($1, 'google', $2, $3)`, userID, subject, email); err != nil {
		t.Fatalf("link google identity: %v", err)
	}
}

func TestCurrentUserReportsTheSessionShape(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool, admin)

	user, err := svc.CurrentUser(context.Background(), id)
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if user.Email != email || user.Role != "admin" {
		t.Errorf("got %s/%s, want %s/admin", user.Email, user.Role, email)
	}
	if !user.HasPassword() {
		t.Error("hasPassword = false for a password account")
	}
	if len(user.LinkedProviders) != 0 {
		t.Errorf("linkedProviders = %v, want empty", user.LinkedProviders)
	}
	if user.MustChangePassword {
		t.Error("mustChangePassword = true unexpectedly")
	}
}

func TestAGoogleOnlyUserReportsNoPasswordAndTheGoogleProvider(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool, googleOnly)
	linkGoogle(t, pool, id, email)

	user, err := svc.CurrentUser(context.Background(), id)
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if user.HasPassword() {
		t.Error("hasPassword = true for a Google-only account")
	}
	if len(user.LinkedProviders) != 1 || user.LinkedProviders[0] != "google" {
		t.Errorf("linkedProviders = %v, want [google]", user.LinkedProviders)
	}
}

func TestASuspendedAccountHasNoCurrentUser(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool, disabled)

	if _, err := svc.CurrentUser(context.Background(), id); !errors.Is(err, auth.ErrAccountDisabled) {
		t.Fatalf("error = %v, want ErrAccountDisabled", err)
	}
}

func TestCurrentUserRejectsAnIdThatNoLongerExists(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)

	_, err := svc.CurrentUser(context.Background(), "00000000-0000-7000-8000-000000000000")
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}
