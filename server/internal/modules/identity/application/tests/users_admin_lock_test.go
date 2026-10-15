//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

type editInFlight struct {
	tx  pgx.Tx
	pid int
}

func (w *usersWorld) beginEdit(t *testing.T) *editInFlight {
	t.Helper()
	ctx := context.Background()
	tx, err := w.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	edit := &editInFlight{tx: tx}
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&edit.pid); err != nil {
		t.Fatal(err)
	}
	return edit
}

func (e *editInFlight) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (e *editInFlight) commit(t *testing.T) {
	t.Helper()
	if err := e.tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type createOutcome struct {
	result command.CreateUserResult
	err    error
}

func (w *usersWorld) createInBackground(req domain.WriteRequest, in domain.NewUser) <-chan createOutcome {
	done := make(chan createOutcome, 1)
	go func() {
		result, err := w.create(req, in)
		done <- createOutcome{result, err}
	}()
	return done
}

func (w *usersWorld) waitUntilHeldBackBy(t *testing.T, edit *editInFlight, done <-chan createOutcome) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case out := <-done:
			t.Fatalf("the create returned (%v) while an edit it depends on was still uncommitted; it must wait for it", out.err)
		default:
		}
		var waiting int
		if err := w.admin.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))`, edit.pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the create was never held back by the edit in flight")
}

func settled(t *testing.T, done <-chan createOutcome) createOutcome {
	t.Helper()
	select {
	case out := <-done:
		return out
	case <-time.After(10 * time.Second):
		t.Fatal("the create did not finish after the edit committed")
		return createOutcome{}
	}
}

func TestACreateWaitsForARoleEditInFlightAndThenSeesTheEditedRole(t *testing.T) {
	w := newUsersWorld(t, true)
	role := w.customRole(t, access.TeachingGrading)
	edit := w.beginEdit(t)
	edit.exec(t, `UPDATE app.roles SET revision = revision + 1 WHERE id = $1::uuid`, role)
	edit.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, role, string(access.ContentShare))
	in := domain.NewUser{Email: w.email("ct"), FullName: "Bùi Thanh Sơn", RoleID: role, Delivery: domain.DeliveryEmailLink}

	done := w.createInBackground(w.peopleOnly(), in)
	w.waitUntilHeldBackBy(t, edit, done)
	edit.commit(t)

	if out := settled(t, done); !errors.Is(out.err, accessdomain.ErrTargetRoleNotAllowed) {
		t.Fatalf("error = %v, want the subset rule's refusal for a role that grew while the create waited", out.err)
	}
	w.nothingWritten(t, in.Email)
}

func TestACreateWaitsForAClassArchiveInFlightAndThenFindsNoClass(t *testing.T) {
	w := newUsersWorld(t, true)
	class := w.class(t, false)
	edit := w.beginEdit(t)
	edit.exec(t, `UPDATE app.classes SET archived_at = now() WHERE id = $1::uuid`, class)
	in := domain.NewUser{Email: w.email("cl"), FullName: "Lưu Bảo Châu", RoleID: w.role(t, "student"), ClassID: class, Delivery: domain.DeliveryEmailLink}

	done := w.createInBackground(w.everything(), in)
	w.waitUntilHeldBackBy(t, edit, done)
	edit.commit(t)

	if out := settled(t, done); !errors.Is(out.err, domain.ErrClassNotFound) {
		t.Fatalf("error = %v, want ErrClassNotFound for a class archived while the create waited", out.err)
	}
	w.nothingWritten(t, in.Email)
}
