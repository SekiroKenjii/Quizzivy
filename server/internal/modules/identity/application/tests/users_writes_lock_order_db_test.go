//go:build integration

package application_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/shared/access"
)

func (w *writesWorld) waitBlockedBy(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		if err := w.admin.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE $1::int = ANY(pg_blocking_pids(pid))`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the write never waited on the held row")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAPromotionWaitingOnAnAdminRowDoesNotDeadlockTheDisableThatHoldsIt(t *testing.T) {
	w := newWritesWorld(t)
	ctx := context.Background()
	target := w.account(t, "pr", "teacher")
	held := w.account(t, "qt", "admin")
	var adminRole string
	if err := w.admin.QueryRow(ctx, `SELECT id::text FROM app.roles WHERE builtin_key = 'admin'`).Scan(&adminRole); err != nil {
		t.Fatal(err)
	}

	tx, err := w.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, held); err != nil {
		t.Fatal(err)
	}

	promoted := make(chan error, 1)
	go func() { promoted <- w.changeRole(w.everything(), target, adminRole) }()

	w.waitBlockedBy(t, pid)
	var detector float64
	if err := w.admin.QueryRow(ctx, `SELECT extract(epoch FROM current_setting('deadlock_timeout')::interval)`).Scan(&detector); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Duration(detector*float64(time.Second)) + 250*time.Millisecond)

	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '5s'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, held); err != nil {
		t.Fatalf("the disable holding the Admin row: %v, want it to go through while the promotion waits", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-promoted:
		if err != nil {
			t.Fatalf("the promotion: %v, want it made once the disable committed", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the promotion never finished")
	}
	if got := w.row(t, target); got.RoleID != adminRole {
		t.Errorf("the promoted account holds role %s, want the Admin role", got.RoleID)
	}
	if got := w.row(t, held); !got.Disabled {
		t.Error("the held Admin is not disabled")
	}
}

func TestASignOutWaitingOnARoleChangeSignsTheAccountOutUnderItsNewRole(t *testing.T) {
	w := newWritesWorld(t)
	ctx := context.Background()
	target := w.account(t, "so", "teacher")
	w.holdSessions(t, target, 1)
	grader := w.customRole(t, access.TeachingGrading)

	tx, err := w.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.users SET role_id = $2::uuid WHERE id = $1::uuid`, target, grader); err != nil {
		t.Fatal(err)
	}

	signedOut := make(chan error, 1)
	go func() {
		_, err := w.app.Commands.SignOutUser.Handle(ctx, command.SignOutUser{Request: w.everything(), ID: target})
		signedOut <- err
	}()
	w.waitBlockedBy(t, pid)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-signedOut:
		if err != nil {
			t.Fatalf("the sign-out: %v, want it made under the new role", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the sign-out never finished")
	}
	if got := w.row(t, target); got.RoleID != grader || got.LiveSessions != 0 {
		t.Errorf("after = %+v, want the new role and no session", got)
	}
}
