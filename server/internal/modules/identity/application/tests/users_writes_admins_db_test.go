//go:build integration

package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
)

func newAdminsWorld(t *testing.T) (w *writesWorld, appdb *pgxpool.Pool, first, other string) {
	t.Helper()
	admin, appdb := scratchPools(t)
	w = withPrincipals(newUsersWorldOn(t, admin, appdb, true, false))
	return w, appdb, w.account(t, "qt1", "admin"), w.account(t, "qt2", "admin")
}

func (w *writesWorld) adminRequest(actor string) domain.WriteRequest {
	req := w.everything()
	req.ActorID = actor
	return req
}

func (w *writesWorld) activeAdmins(t *testing.T) int {
	t.Helper()
	return w.count(t, `SELECT count(*) FROM app.users u JOIN app.roles r ON r.id = u.role_id WHERE r.builtin_key = 'admin' AND u.disabled_at IS NULL`)
}

func TestTheLastActiveAdminCanNeitherBeDisabledNorDemotedAndADisabledAdminDoesNotCount(t *testing.T) {
	w, _, first, other := newAdminsWorld(t)
	teacherRole := w.role(t, "teacher")
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, other)
	before := w.row(t, first)

	if err := w.disable(w.everything(), first); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("disabling the only active Admin: %v, want ErrLastAdmin", err)
	}
	if err := w.changeRole(w.everything(), first, teacherRole); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("demoting the only active Admin: %v, want ErrLastAdmin", err)
	}
	if after := w.row(t, first); after != before {
		t.Errorf("refusals changed %+v into %+v", before, after)
	}
	if n := len(w.audits(t, first, "user.disabled")) + len(w.audits(t, first, "user.role_changed")); n != 0 {
		t.Errorf("refusals wrote %d audit rows", n)
	}

	if _, err := w.app.Commands.EnableUser.Handle(context.Background(), command.EnableUser{Request: w.everything(), ID: other}); err != nil {
		t.Fatal(err)
	}
	if err := w.changeRole(w.everything(), first, teacherRole); err != nil {
		t.Fatalf("demoting an Admin when another is active: %v", err)
	}
	if err := w.disable(w.everything(), other); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("disabling the Admin who is now the only one: %v, want ErrLastAdmin", err)
	}
	if got := w.activeAdmins(t); got != 1 {
		t.Errorf("%d active Admins, want 1", got)
	}
}

func TestTheDatabaseRefusesToEndTheLastAdminFromTheAppRole(t *testing.T) {
	w, appdb, first, other := newAdminsWorld(t)
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, other)
	ctx := context.Background()
	teacherRole := w.role(t, "teacher")

	for name, statement := range map[string]string{
		"disable": `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`,
		"demote":  `UPDATE app.users SET role_id = '` + teacherRole + `'::uuid WHERE id = $1::uuid`,
		"delete":  `DELETE FROM app.users WHERE id = $1::uuid`,
	} {
		_, err := appdb.Exec(ctx, statement, first)
		if !db.IsCheckViolation(err, "users_last_admin") {
			t.Errorf("%s as quizzivy_app: %v, want the users_last_admin check", name, err)
			continue
		}
		if !errors.Is(repositories.UserWriteError(err), domain.ErrLastAdmin) {
			t.Errorf("%s: the repository does not map %v to ErrLastAdmin", name, err)
		}
	}
	if got := w.activeAdmins(t); got != 1 {
		t.Errorf("%d active Admins, want 1", got)
	}
}

func TestTwoAdminsEndingEachOthersAccessAtOnceLeaveOneActive(t *testing.T) {
	w, _, first, other := newAdminsWorld(t)
	teacherRole := w.role(t, "teacher")
	type act struct {
		name string
		a, b func() error
	}
	for _, c := range []act{
		{"both disable the other",
			func() error { return w.disable(w.adminRequest(first), other) },
			func() error { return w.disable(w.adminRequest(other), first) }},
		{"both demote the other",
			func() error { return w.changeRole(w.adminRequest(first), other, teacherRole) },
			func() error { return w.changeRole(w.adminRequest(other), first, teacherRole) }},
		{"one disables while the other demotes",
			func() error { return w.disable(w.adminRequest(first), other) },
			func() error { return w.changeRole(w.adminRequest(other), first, teacherRole) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			for round := range 12 {
				w.exec(t, `UPDATE app.users SET disabled_at = NULL, role_id = (SELECT id FROM app.roles WHERE builtin_key = 'admin') WHERE id IN ($1::uuid, $2::uuid)`, first, other)
				var wg sync.WaitGroup
				start := make(chan struct{})
				errs := make([]error, 2)
				for i, run := range []func() error{c.a, c.b} {
					wg.Add(1)
					go func() {
						defer wg.Done()
						<-start
						errs[i] = run()
					}()
				}
				close(start)
				wg.Wait()

				wins, refused := 0, 0
				for _, err := range errs {
					switch {
					case err == nil:
						wins++
					case errors.Is(err, domain.ErrLastAdmin):
						refused++
					default:
						t.Fatalf("round %d: %v", round, err)
					}
				}
				if wins != 1 || refused != 1 || w.activeAdmins(t) != 1 {
					t.Fatalf("round %d: %d succeeded, %d refused, %d Admins active, want one of each and one Admin left: %v", round, wins, refused, w.activeAdmins(t), errs)
				}
			}
		})
	}
}
