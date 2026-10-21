//go:build integration

package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/shared/access"
)

func (w *bulkWorld) setMustChange(t *testing.T, value bool, ids ...string) {
	t.Helper()
	w.exec(t, `UPDATE app.users SET must_change_password = $2::boolean WHERE id = ANY($1::uuid[])`, ids, value)
}

func (w *bulkWorld) createAs(t *testing.T, name string) domain.Student {
	t.Helper()
	created, err := w.app.Commands.CreateStudent.Handle(context.Background(), command.CreateStudent{
		Request: domain.WriteRequest{ActorID: w.a, Grants: access.NewSet(access.PeopleStudentsCreate)},
		Input:   domain.NewStudent{Email: uuid.NewString() + "@example.test", FullName: name + " " + w.marker},
	})
	if err != nil {
		t.Fatalf("CreateStudent: %v", err)
	}
	return created.Student
}

func (w *bulkWorld) resetOneAs(t *testing.T, id string) {
	t.Helper()
	if _, err := w.app.Commands.ResetStudentPassword.Handle(context.Background(), command.ResetStudentPassword{Request: w.requestA, ID: id}); err != nil {
		t.Fatalf("ResetStudentPassword: %v", err)
	}
}

func TestTemporaryPasswordsAreStoredForcedOrKeptAsTheCentreDecides(t *testing.T) {
	for name, c := range map[string]struct {
		policy ports.SignInPolicy
		forced bool
	}{
		"with no policy port":        {nil, true},
		"with the change forced":     {policyOf(openPolicy()), true},
		"with the change not forced": {policyOf(policyWith(keepTemporary)), false},
	} {
		t.Run(name, func(t *testing.T) {
			w := newBulkWorld(t)
			w.app.SetSignInPolicy(c.policy)
			single, bulk := w.mine[0], []string{w.mine[1], w.mine[2]}
			w.setMustChange(t, !c.forced, single, bulk[0], bulk[1])
			w.setMustChange(t, true, w.extra)
			w.setMustChange(t, false, w.shared, w.theirs)
			bystanders := map[string]accountState{w.extra: w.state(t, w.extra), w.shared: w.state(t, w.shared), w.theirs: w.state(t, w.theirs)}

			created := w.createAs(t, "Học sinh mới")
			w.resetOneAs(t, single)
			result := w.resetAs(t, w.requestA, bulk...)

			if created.MustChangePassword != c.forced {
				t.Errorf("the created student reads mustChangePassword = %v, want %v", created.MustChangePassword, c.forced)
			}
			if got := w.state(t, created.ID).MustChange; got != c.forced {
				t.Errorf("the created student is stored with must_change_password = %v, want %v", got, c.forced)
			}
			if got := w.state(t, single).MustChange; got != c.forced {
				t.Errorf("a single reset stored must_change_password = %v, want %v", got, c.forced)
			}
			if len(result.Reset) != len(bulk) {
				t.Fatalf("the bulk reset reset %d students (failed %d), want %d", len(result.Reset), len(result.Failed), len(bulk))
			}
			for _, id := range bulk {
				if got := w.state(t, id).MustChange; got != c.forced {
					t.Errorf("a bulk reset stored must_change_password = %v for %s, want %v", got, id, c.forced)
				}
			}
			for id, before := range bystanders {
				if after := w.state(t, id); after != before {
					t.Errorf("a student outside the writes changed: %+v became %+v", before, after)
				}
			}
		})
	}
}

func TestChangingThePolicyLeavesTheFlagsAlreadyIssuedAsTheyWere(t *testing.T) {
	w := newBulkWorld(t)
	current := openPolicy()
	w.app.SetSignInPolicy(switchablePolicy(func() organizationmodel.SignInPolicy { return current }))

	forcedCreate := w.createAs(t, "Được ép đổi")
	w.resetOneAs(t, w.mine[0])
	current = policyWith(keepTemporary)
	keptCreate := w.createAs(t, "Được giữ")
	w.resetOneAs(t, w.mine[1])

	for id, want := range map[string]bool{forcedCreate.ID: true, w.mine[0]: true, keptCreate.ID: false, w.mine[1]: false} {
		if got := w.state(t, id).MustChange; got != want {
			t.Errorf("%s stores must_change_password = %v, want %v", id, got, want)
		}
	}

	current = openPolicy()
	if got := w.state(t, keptCreate.ID).MustChange; got {
		t.Error("turning the forced change back on set the flag of a temporary password already issued")
	}
	if got := w.state(t, w.mine[1]).MustChange; got {
		t.Error("turning the forced change back on set the flag of a reset already made")
	}
}
