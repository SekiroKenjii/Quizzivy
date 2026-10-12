//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application/command"
)

func TestARefreshCancelledBeforeItsCommitLeavesThePresentedTokenLive(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	token := login(t, svc, email)
	family := loadToken(t, pool, token).familyID

	rotation, pid := heldRotation(t, pool, id, token)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	refreshed := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Refresh.Handle(ctx, command.Refresh{Token: token})
		refreshed <- err
	}()
	waitUntilRevocationIsBlockedBy(t, pool, pid, refreshed)

	cancel()
	select {
	case err := <-refreshed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("a cancelled refresh answered %v, want an error that is context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a cancelled refresh kept waiting for the lock")
	}
	if err := rotation.Rollback(context.Background()); err != nil {
		t.Fatalf("release the lock: %v", err)
	}

	row := loadToken(t, pool, token)
	if row.revoked || row.replacedBy != nil {
		t.Errorf("the presented token after a cancelled refresh = %+v, want it live and unreplaced: the client never saw a successor", row)
	}
	if n := liveTokensInFamily(t, pool, family); n != 1 {
		t.Errorf("live tokens in the family = %d, want 1: no successor may exist for a rotation nobody received", n)
	}
	if _, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: token}); err != nil {
		t.Errorf("the client's retry with the same token answered %v, want it rotated", err)
	}
}

func TestALogoutTheClientAbandonsWhileItWaitsStillRevokesTheFamily(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	token := login(t, svc, email)
	family := loadToken(t, pool, token).familyID

	rotation, pid := heldRotation(t, pool, id, token)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	loggedOut := make(chan error, 1)
	go func() {
		_, err := svc.Commands.Logout.Handle(ctx, command.Logout{Token: token})
		loggedOut <- err
	}()
	waitUntilRevocationIsBlockedBy(t, pool, pid, loggedOut)

	cancel()
	if err := rotation.Rollback(context.Background()); err != nil {
		t.Fatalf("release the lock: %v", err)
	}

	select {
	case err := <-loggedOut:
		if err != nil {
			t.Errorf("a logout whose client hung up answered %v, want the revoke finished", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the logout did not finish after the lock was released")
	}
	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens after an abandoned logout = %d, want 0: the cookie stays in the browser and would sign the next visitor in", n)
	}
}

func TestALogoutWhoseClientLeftBeforeItStartedStillRevokesTheFamily(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	token := login(t, svc, email)
	family := loadToken(t, pool, token).familyID

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Commands.Logout.Handle(gone, command.Logout{Token: token}); err != nil {
		t.Errorf("a logout on a cancelled request answered %v, want the revoke done", err)
	}
	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens = %d, want 0", n)
	}
}
