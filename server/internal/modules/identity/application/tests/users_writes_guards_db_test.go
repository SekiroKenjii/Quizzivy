//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

type roleAt struct {
	id       string
	revision int64
}

func (w *writesWorld) builtinRole(t *testing.T, key string) roleAt {
	t.Helper()
	var r roleAt
	if err := w.admin.QueryRow(context.Background(), `SELECT id::text, revision FROM app.roles WHERE builtin_key = $1`, key).Scan(&r.id, &r.revision); err != nil {
		t.Fatal(err)
	}
	return r
}

func (w *writesWorld) roleOf(t *testing.T, q db.Querier, id string) string {
	t.Helper()
	var role string
	if err := q.QueryRow(context.Background(), `SELECT role_id::text FROM app.users WHERE id = $1::uuid`, id).Scan(&role); err != nil {
		t.Fatal(err)
	}
	return role
}

func demotion(id, role string, to roleAt) domain.RoleChange {
	return domain.RoleChange{
		AccountWrite: domain.AccountWrite{UserID: id, RoleID: role, Now: time.Now()},
		ToRoleID:     to.id, ToRoleName: "Giáo viên", ToRoleRevision: to.revision, ToOpensTeacher: true,
	}
}

func TestWithTwoAdminsLeftOneMayGoAndTheLastMayNot(t *testing.T) {
	w, _, first, second := newAdminsWorld(t)
	ctx := context.Background()
	teacher := w.insertUser(t, "two-t", "teacher")
	admin, teaching := w.builtinRole(t, "admin"), w.builtinRole(t, "teacher")
	repo := repositories.NewAdminUsers(w.dbx, w.links)

	if err := repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: teacher, RoleID: teaching.id, Now: time.Now()}); err != nil {
		t.Fatalf("disabling a teacher beside two Admins: %v", err)
	}
	if err := repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: second, RoleID: admin.id, Now: time.Now()}); err != nil {
		t.Fatalf("disabling one of two Admins: %v, want it done: the other stays", err)
	}
	if err := repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: first, RoleID: admin.id, Now: time.Now()}); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("disabling the last Admin: %v, want ErrLastAdmin", err)
	}
	if err := repo.ChangeRole(ctx, w.everything(), demotion(first, admin.id, teaching)); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("demoting the last Admin: %v, want ErrLastAdmin", err)
	}
	var active int
	if err := w.admin.QueryRow(ctx, `
		SELECT count(*)::int FROM app.users WHERE role_id = $1::uuid AND disabled_at IS NULL`, admin.id).Scan(&active); err != nil || active != 1 {
		t.Errorf("%d active Admins (%v), want the one left", active, err)
	}
}

func TestTwoOfTwoAdminsDemotingEachOtherLeaveOne(t *testing.T) {
	w, _, first, second := newAdminsWorld(t)
	ctx := context.Background()
	admin, teaching := w.builtinRole(t, "admin"), w.builtinRole(t, "teacher")
	repo := repositories.NewAdminUsers(w.dbx, w.links)

	if err := repo.ChangeRole(ctx, w.everything(), demotion(second, admin.id, teaching)); err != nil {
		t.Fatalf("demoting one of two Admins: %v", err)
	}
	if err := repo.ChangeRole(ctx, w.everything(), demotion(first, admin.id, teaching)); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("demoting the other: %v, want ErrLastAdmin", err)
	}
}

func TestThreeAdminsActingOnEachOtherInARingNeverDeadlock(t *testing.T) {
	w, _, _, _ := newAdminsWorld(t)
	ctx := context.Background()
	admin, teaching := w.builtinRole(t, "admin"), w.builtinRole(t, "teacher")
	repo := repositories.NewAdminUsers(w.dbx, w.links)
	for round := range 30 {
		ring := []string{
			w.insertUser(t, fmt.Sprintf("ring%d-a", round), "admin"),
			w.insertUser(t, fmt.Sprintf("ring%d-b", round), "admin"),
			w.insertUser(t, fmt.Sprintf("ring%d-c", round), "admin"),
		}
		start := make(chan struct{})
		errs := make([]error, len(ring))
		var group sync.WaitGroup
		for i, id := range ring {
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				if i == 1 {
					errs[i] = repo.ChangeRole(ctx, w.everything(), demotion(id, admin.id, teaching))
					return
				}
				errs[i] = repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: id, RoleID: admin.id, Now: time.Now()})
			}()
		}
		close(start)
		group.Wait()
		for i, err := range errs {
			if errors.Is(err, domain.ErrWriteContended) {
				t.Fatalf("round %d, account %d: %v; the writes lock in different orders", round, i, err)
			}
			if err != nil && !errors.Is(err, domain.ErrLastAdmin) {
				t.Fatalf("round %d, account %d: %v", round, i, err)
			}
		}
	}
}

func TestARoleChangeTheDatabaseEndsInADeadlockIsErrWriteContended(t *testing.T) {
	w, _, _, _ := newAdminsWorld(t)
	ctx := context.Background()
	target := w.insertUser(t, "cr-t", "teacher")
	last := w.insertUser(t, "cr-l", "admin")
	var first string
	if err := w.admin.QueryRow(ctx, `
		SELECT u.id::text FROM app.users u
		 WHERE (u.role_id = (SELECT r.id FROM app.roles r WHERE r.builtin_key = 'admin') AND u.disabled_at IS NULL) OR u.id = $1::uuid
		 ORDER BY u.id LIMIT 1`, target).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if first == last {
		t.Fatal("the test needs an account that sorts before the last Admin")
	}
	role := w.roleOf(t, w.admin, target)
	student := w.builtinRole(t, "student")
	holder, pid := w.lockedIn(t, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, last)

	repo := repositories.NewAdminUsers(w.dbx, w.links)
	written := make(chan error, 1)
	go func() {
		written <- repo.ChangeRole(ctx, w.everything(), demotion(target, role, student))
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
		t.Fatal("the role change never finished")
	}
	var pg *pgconn.PgError
	if !errors.Is(err, domain.ErrWriteContended) || !errors.As(err, &pg) || pg.Code != pgerrcode.DeadlockDetected {
		t.Fatalf("err = %v, want ErrWriteContended wrapping the database's 40P01", err)
	}
	if got := w.roleOf(t, w.admin, target); got != role {
		t.Errorf("the account holds role %s after the lost write, want %s", got, role)
	}
}

func TestADatabaseErrorThatIsNotContentionIsNotRetriedAsOne(t *testing.T) {
	w := newWritesWorld(t)
	ctx := context.Background()
	target := w.account(t, "lt-t", "teacher")
	role := w.roleOf(t, w.admin, target)
	w.lockedIn(t, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, target)
	tx, err := w.dbx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '200ms'`); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewAdminUsers(db.NewContext(tx), w.links)

	for name, write := range map[string]func() error{
		"disable": func() error {
			return repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: target, RoleID: role, Now: time.Now()})
		},
		"role change": func() error {
			return repo.ChangeRole(ctx, w.everything(), demotion(target, role, w.builtinRole(t, "student")))
		},
	} {
		err := write()
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != pgerrcode.LockNotAvailable {
			t.Errorf("%s: %v, want the lock timeout", name, err)
		}
		if errors.Is(err, domain.ErrWriteContended) {
			t.Errorf("%s: a lock timeout became ErrWriteContended and would be retried", name)
		}
	}
}

func TestTheAdminsAreLockedInIdOrderAndNotInTheOrderTheTableHoldsThem(t *testing.T) {
	w, _, _, _ := newAdminsWorld(t)
	ctx := context.Background()
	lower, higher := w.insertUser(t, "io-a", "admin"), w.insertUser(t, "io-b", "admin")
	target := w.insertUser(t, "io-t", "teacher")
	if lower >= higher || higher >= target {
		t.Fatalf("ids %s, %s, %s are not in creation order", lower, higher, target)
	}
	w.placeAfter(t, lower, higher)
	holder, pid := w.lockedIn(t, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, lower)

	disabled := make(chan error, 1)
	go func() { disabled <- w.disable(w.everything(), target) }()
	w.waitBlockedBy(t, pid)

	probe, err := w.admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, probed := probe.Exec(ctx, `SELECT 1 FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE NOWAIT`, higher)
	_ = probe.Rollback(ctx)
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
	if probed != nil {
		t.Errorf("an Admin above the one the disable waits on was already locked (%v): the statement locks in table order, not id order", probed)
	}
}

func TestAWriteInARepeatableReadTransactionThatLosesItsSnapshotIsErrWriteContended(t *testing.T) {
	w := newWritesWorld(t)
	ctx := context.Background()
	target := w.insertUser(t, "sf-t", "teacher")
	role := w.roleOf(t, w.admin, target)
	tx, err := w.admin.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(ctx, `SELECT 1 FROM app.roles LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	w.exec(t, `UPDATE app.users SET full_name = full_name || ' x' WHERE id = $1::uuid`, target)
	repo := repositories.NewAdminUsers(db.NewContext(tx), w.links)

	err = repo.Disable(ctx, w.everything(), domain.AccountWrite{UserID: target, RoleID: role, Now: time.Now()})

	var pg *pgconn.PgError
	if !errors.Is(err, domain.ErrWriteContended) || !errors.As(err, &pg) || pg.Code != pgerrcode.SerializationFailure {
		t.Fatalf("err = %v, want ErrWriteContended wrapping the database's 40001", err)
	}
}
