//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func heldRotation(t *testing.T, pool *pgxpool.Pool, userID, token string) (pgx.Tx, int) {
	t.Helper()
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the rotation: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("roll back the rotation: %v", err)
		}
	})

	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("read the rotation's pid: %v", err)
	}

	var locked bool
	if err := tx.QueryRow(ctx,
		`SELECT true FROM app.users WHERE id = $1 FOR SHARE`, userID).Scan(&locked); err != nil {
		t.Fatalf("lock the token's owner: %v", err)
	}

	presented := sha256.Sum256([]byte(token))
	var predecessor string
	if err := tx.QueryRow(ctx,
		`SELECT id FROM app.refresh_tokens WHERE token_hash = $1 FOR UPDATE`, presented[:]).Scan(&predecessor); err != nil {
		t.Fatalf("claim the presented token: %v", err)
	}

	next := make([]byte, sha256.Size)
	if _, err := rand.Read(next); err != nil {
		t.Fatalf("mint the successor's hash: %v", err)
	}
	var successor string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, issued_at, expires_at)
		 SELECT user_id, family_id, $2::bytea, now(), now() + interval '1 day'
		   FROM app.refresh_tokens WHERE token_hash = $1
		 RETURNING id`, presented[:], next).Scan(&successor); err != nil {
		t.Fatalf("issue the successor: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE app.refresh_tokens SET revoked_at = now(), replaced_by = $2 WHERE token_hash = $1`,
		presented[:], successor); err != nil {
		t.Fatalf("consume the presented token: %v", err)
	}
	return tx, pid
}

func waitUntilRevocationIsBlockedBy(t *testing.T, pool *pgxpool.Pool, pid int, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		if err := pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked); err != nil {
			t.Fatalf("look for the waiting revocation: %v", err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the revocation never waited on the rotation")
		}
		select {
		case err := <-done:
			t.Fatalf("the revocation finished while a rotation was in flight: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestALogoutWaitsForARotationInFlightAndRevokesItsSuccessor(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	token := login(t, svc, email)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rotation, pid := heldRotation(t, pool, id, token)

	loggedOut := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Logout.Handle(ctx, command.Logout{Token: token})
		loggedOut <- err
	}()

	waitUntilRevocationIsBlockedBy(t, pool, pid, loggedOut)

	if err := rotation.Commit(ctx); err != nil {
		t.Fatalf("commit the rotation: %v", err)
	}

	select {
	case err := <-loggedOut:
		if err != nil {
			t.Fatalf("logout: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the logout did not return after the rotation committed")
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, id).Scan(&live); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	if live != 0 {
		t.Errorf("live tokens after the logout = %d, want 0", live)
	}
}

func TestAReuseDetectionWaitsForARotationInFlightAndRevokesItsSuccessor(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	first := login(t, svc, email)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	second, err := svc.Commands.Refresh.Handle(ctx, command.Refresh{Token: first})
	if err != nil {
		t.Fatalf("rotate the first token: %v", err)
	}

	rotation, pid := heldRotation(t, pool, id, second.RefreshToken)

	replayed := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Refresh.Handle(ctx, command.Refresh{Token: first})
		replayed <- err
	}()

	waitUntilRevocationIsBlockedBy(t, pool, pid, replayed)

	if err := rotation.Commit(ctx); err != nil {
		t.Fatalf("commit the rotation: %v", err)
	}

	select {
	case err := <-replayed:
		if !errors.Is(err, domain.ErrRefreshReused) {
			t.Fatalf("replay error = %v, want ErrRefreshReused", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the replay did not return after the rotation committed")
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, id).Scan(&live); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	if live != 0 {
		t.Errorf("live tokens after the reuse detection = %d, want 0", live)
	}
}
