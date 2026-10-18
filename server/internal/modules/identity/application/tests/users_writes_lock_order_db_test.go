//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
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

func (w *writesWorld) lockedIn(t *testing.T, sql string, args ...any) (pgx.Tx, int) {
	t.Helper()
	ctx := context.Background()
	tx, err := w.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
	return tx, pid
}

func (w *writesWorld) sleepPastTheDeadlockDetector(t *testing.T) {
	t.Helper()
	var detector float64
	if err := w.admin.QueryRow(context.Background(), `SELECT extract(epoch FROM current_setting('deadlock_timeout')::interval)`).Scan(&detector); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Duration(detector*float64(time.Second)) + 250*time.Millisecond)
}

func (w *writesWorld) sleepHalfTheDeadlockDetector(t *testing.T) {
	t.Helper()
	var detector float64
	if err := w.admin.QueryRow(context.Background(), `SELECT extract(epoch FROM current_setting('deadlock_timeout')::interval)`).Scan(&detector); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Duration(detector * float64(time.Second) / 2))
}

func (w *writesWorld) placeAfter(t *testing.T, moved, fixed string) {
	t.Helper()
	for range 50 {
		var after bool
		if err := w.admin.QueryRow(context.Background(), `
			SELECT (SELECT ctid FROM app.users WHERE id = $1::uuid) > (SELECT ctid FROM app.users WHERE id = $2::uuid)`, moved, fixed).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after {
			return
		}
		w.exec(t, `UPDATE app.users SET email = 'm' || email WHERE id = $1::uuid`, moved)
	}
	t.Fatalf("account %s never moved past %s in the table", moved, fixed)
}

func TestADisableLocksItsTargetInIdOrderAmongTheAdmins(t *testing.T) {
	w := newWritesWorld(t)
	ctx := context.Background()
	target := w.account(t, "dt", "teacher")
	above := w.account(t, "da", "admin")
	if above <= target {
		t.Fatalf("the Admin %s does not sort after the target %s", above, target)
	}
	holder, pid := w.lockedIn(t, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, target)

	disabled := make(chan error, 1)
	go func() { disabled <- w.disable(w.everything(), target) }()
	w.waitBlockedBy(t, pid)
	w.sleepPastTheDeadlockDetector(t)

	if _, err := holder.Exec(ctx, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, above); err != nil {
		t.Fatalf("a transaction holding the target and then taking an Admin above it: %v, want no deadlock", err)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-disabled:
		if err != nil {
			t.Fatalf("the disable: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the disable never finished")
	}
	if !w.row(t, target).Disabled {
		t.Error("the target is not disabled")
	}
}

func TestADeadlockTheRepositoryLosesIsErrWriteContendedWithTheDatabaseErrorKept(t *testing.T) {
	w, _, _, _ := newAdminsWorld(t)
	ctx := context.Background()
	target := w.insertUser(t, "rt", "teacher")
	last := w.insertUser(t, "rl", "admin")
	var first, role string
	if err := w.admin.QueryRow(ctx, `
		SELECT u.id::text FROM app.users u
		 WHERE (u.role_id = (SELECT r.id FROM app.roles r WHERE r.builtin_key = 'admin') AND u.disabled_at IS NULL) OR u.id = $1::uuid
		 ORDER BY u.id LIMIT 1`, target).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := w.admin.QueryRow(ctx, `SELECT role_id::text FROM app.users WHERE id = $1::uuid`, target).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if first == last {
		t.Fatal("the test needs an account that sorts before the last Admin")
	}
	holder, pid := w.lockedIn(t, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, last)

	repo := repositories.NewAdminUsers(w.dbx, w.links)
	written := make(chan error, 1)
	go func() {
		written <- repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: target, RoleID: role, Now: time.Now()})
	}()
	w.waitBlockedBy(t, pid)
	w.sleepHalfTheDeadlockDetector(t)
	if _, err := holder.Exec(ctx, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, first); err != nil {
		_ = holder.Rollback(ctx)
		<-written
		t.Fatalf("the holder: %v, want the repository to lose the deadlock it detects first", err)
	}

	var err error
	select {
	case err = <-written:
	case <-time.After(15 * time.Second):
		t.Fatal("the disable never finished")
	}
	var pg *pgconn.PgError
	if !errors.Is(err, domain.ErrWriteContended) || !errors.As(err, &pg) || pg.Code != pgerrcode.DeadlockDetected {
		t.Fatalf("err = %v, want ErrWriteContended wrapping the database's 40P01", err)
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if w.row(t, target).Disabled {
		t.Error("a write the database ended disabled the account")
	}
}
