//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func heldAccountChange(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) (pgx.Tx, int) {
	t.Helper()
	ctx := context.Background()
	change, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the account change: %v", err)
	}
	t.Cleanup(func() {
		if err := change.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("roll back the account change: %v", err)
		}
	})
	if _, err := change.Exec(ctx, statement, args...); err != nil {
		t.Fatalf("change the account: %v", err)
	}
	var pid int
	if err := change.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("read the account change's backend: %v", err)
	}
	return change, pid
}

func waitUntilSignInIsBlockedBy(t *testing.T, pool *pgxpool.Pool, pid int, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		if err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked); err != nil {
			t.Fatalf("look for the waiting sign-in: %v", err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the sign-in never waited on the user")
		}
		select {
		case err := <-done:
			t.Fatalf("the sign-in finished while an account change held the user: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestASignInWaitsForAResetInFlightAndIsRefused(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	temporary, err := domain.Passwords.Hash(ctx, "mật-khẩu-tạm-thời")
	if err != nil {
		t.Fatalf("hash the temporary password: %v", err)
	}
	reset, pid := heldAccountChange(t, pool,
		`UPDATE app.users SET password_hash = $2, must_change_password = true, session_epoch = session_epoch + 1 WHERE id = $1`,
		id, temporary)

	signedIn := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Login.Handle(ctx, command.Login{Email: email, Password: testPassword})
		signedIn <- err
	}()
	waitUntilSignInIsBlockedBy(t, pool, pid, signedIn)

	if _, err := reset.Exec(ctx,
		`UPDATE app.refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
		t.Fatalf("revoke the sessions: %v", err)
	}
	if err := reset.Commit(ctx); err != nil {
		t.Fatalf("commit the reset: %v", err)
	}

	select {
	case err := <-signedIn:
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("sign-in error = %v, want ErrInvalidCredentials", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the sign-in did not return after the reset committed")
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, id).Scan(&live); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	if live != 0 {
		t.Errorf("live tokens after the reset = %d, want 0", live)
	}
}

func TestASignInThatOverlapsTheUsersOwnPasswordChangeIsRefused(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chosen, err := domain.Passwords.Hash(ctx, "mật-khẩu-mới-của-tôi")
	if err != nil {
		t.Fatalf("hash the new password: %v", err)
	}
	change, pid := heldAccountChange(t, pool,
		`UPDATE app.users SET password_hash = $2 WHERE id = $1`, id, chosen)

	signedIn := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Login.Handle(ctx, command.Login{Email: email, Password: testPassword})
		signedIn <- err
	}()
	waitUntilSignInIsBlockedBy(t, pool, pid, signedIn)

	if _, err := change.Exec(ctx,
		`UPDATE app.refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
		t.Fatalf("revoke the sessions: %v", err)
	}
	if err := change.Commit(ctx); err != nil {
		t.Fatalf("commit the password change: %v", err)
	}

	select {
	case err := <-signedIn:
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("sign-in error = %v, want ErrInvalidCredentials", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the sign-in did not return after the password change committed")
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, id).Scan(&live); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	if live != 0 {
		t.Errorf("live tokens after the password change = %d, want 0", live)
	}
}

func TestAGoogleSignInThatOverlapsADisableIsRefusedAsDisabled(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool, googleOnly)
	identity := verifiedIdentity(email)
	linkGoogleSubject(t, pool, id, identity.Subject, email)
	svc, _ := googleService(t, pool, identity)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	disable, pid := heldAccountChange(t, pool,
		`UPDATE app.users SET disabled_at = now(), session_epoch = session_epoch + 1 WHERE id = $1`, id)

	signedIn := make(chan error, 1)
	go func() {
		_, err := svc.Commands.GoogleSignIn.Handle(ctx, command.GoogleSignIn{
			Code: "c", CodeVerifier: "v", RedirectURI: "https://app.quizzivy.com/cb",
			IP: "203.0.113.11", UserAgent: "go-test",
		})
		signedIn <- err
	}()
	waitUntilSignInIsBlockedBy(t, pool, pid, signedIn)

	if err := disable.Commit(ctx); err != nil {
		t.Fatalf("commit the disable: %v", err)
	}

	select {
	case err := <-signedIn:
		if !errors.Is(err, domain.ErrAccountDisabled) {
			t.Fatalf("sign-in error = %v, want ErrAccountDisabled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the sign-in did not return after the disable committed")
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, id).Scan(&live); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	if live != 0 {
		t.Errorf("live tokens after the disable = %d, want 0", live)
	}
}

func TestAGoogleSignInThatOverlapsAResetSignsInAtTheNewEpoch(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool, googleOnly)
	identity := verifiedIdentity(email)
	linkGoogleSubject(t, pool, id, identity.Subject, email)
	svc, _ := googleService(t, pool, identity)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var epoch int
	if err := pool.QueryRow(ctx,
		`SELECT session_epoch FROM app.users WHERE id = $1`, id).Scan(&epoch); err != nil {
		t.Fatalf("read the session epoch: %v", err)
	}
	reset, pid := heldAccountChange(t, pool,
		`UPDATE app.users SET session_epoch = session_epoch + 1 WHERE id = $1`, id)

	var session model.Session
	signedIn := make(chan error, 1)
	go func() {
		result, err := svc.Commands.GoogleSignIn.Handle(ctx, command.GoogleSignIn{
			Code: "c", CodeVerifier: "v", RedirectURI: "https://app.quizzivy.com/cb",
			IP: "203.0.113.11", UserAgent: "go-test",
		})
		session = result.Session
		signedIn <- err
	}()
	waitUntilSignInIsBlockedBy(t, pool, pid, signedIn)

	if err := reset.Commit(ctx); err != nil {
		t.Fatalf("commit the reset: %v", err)
	}

	select {
	case err := <-signedIn:
		if err != nil {
			t.Fatalf("sign-in: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the sign-in did not return after the reset committed")
	}
	if session.User.SessionEpoch != epoch+1 {
		t.Errorf("session epoch = %d, want %d", session.User.SessionEpoch, epoch+1)
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, id).Scan(&live); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	if live != 1 {
		t.Errorf("live tokens after the sign-in = %d, want 1", live)
	}
}
