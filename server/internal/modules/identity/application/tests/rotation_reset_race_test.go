//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestARefreshWaitsForAResetInFlightAndThenFindsItsTokenRevoked(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	token := login(t, svc, email)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reset, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin the reset: %v", err)
	}
	t.Cleanup(func() {
		if err := reset.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("roll back the reset: %v", err)
		}
	})
	if _, err := reset.Exec(ctx,
		`UPDATE app.users SET must_change_password = true, session_epoch = session_epoch + 1 WHERE id = $1`, id); err != nil {
		t.Fatalf("reset the user: %v", err)
	}

	refreshed := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Refresh.Handle(ctx, command.Refresh{Token: token})
		refreshed <- err
	}()

	const waiters = `
		SELECT count(*) FROM pg_stat_activity
		 WHERE wait_event_type = 'Lock'
		   AND query LIKE '%app.users%FOR NO KEY UPDATE%'
		   AND pid <> pg_backend_pid()`
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(ctx, waiters).Scan(&waiting); err != nil {
			t.Fatalf("look for the waiting refresh: %v", err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the refresh never waited on the user")
		}
		select {
		case err := <-refreshed:
			t.Fatalf("the refresh finished while a reset held the user: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}

	if _, err := reset.Exec(ctx,
		`UPDATE app.refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
		t.Fatalf("revoke the sessions: %v", err)
	}
	if err := reset.Commit(ctx); err != nil {
		t.Fatalf("commit the reset: %v", err)
	}

	select {
	case err := <-refreshed:
		if !errors.Is(err, domain.ErrRefreshRejected) {
			t.Fatalf("refresh error = %v, want ErrRefreshRejected", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the refresh did not return after the reset committed")
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
