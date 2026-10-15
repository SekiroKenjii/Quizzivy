//go:build integration

package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	accessmodel "quizzivy/internal/modules/access/application/model"
	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/shared/access"
)

type changingRoles struct {
	inner interface {
		Handle(context.Context, accessquery.AssignableRole) (accessmodel.AssignableRole, error)
	}
	between func()
	reads   int
}

func (c *changingRoles) Handle(ctx context.Context, q accessquery.AssignableRole) (accessmodel.AssignableRole, error) {
	role, err := c.inner.Handle(ctx, q)
	c.reads++
	if c.between != nil {
		c.between()
	}
	return role, err
}

func (w *usersWorld) customRole(t *testing.T, grants ...access.Key) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := w.admin.QueryRow(ctx, `
		INSERT INTO app.roles (name, icon, color) VALUES ($1, 'key-round', 'gray') RETURNING id::text`,
		"Vai trò thử "+uuid.NewString()[:8]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, stmt := range []string{
			`DELETE FROM app.audit_log WHERE entity_id IN (SELECT id FROM app.users WHERE role_id = $1::uuid)`,
			`DELETE FROM app.users WHERE role_id = $1::uuid`,
			`DELETE FROM app.roles WHERE id = $1::uuid`,
		} {
			if _, err := w.admin.Exec(context.Background(), stmt, id); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
	for _, key := range grants {
		if _, err := w.admin.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, id, string(key)); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func (w *usersWorld) watchingRoles(between func()) *changingRoles {
	hooked := &changingRoles{inner: w.roles, between: between}
	w.app.SetAdminUsers(repositories.NewAdminUsers(w.dbx, w.links), hooked)
	return hooked
}

func TestARoleThatGainsAPermissionAfterTheCheckIsCheckedAgainAndRefused(t *testing.T) {
	w := newUsersWorld(t, true)
	role := w.customRole(t, access.TeachingGrading)
	hooked := w.watchingRoles(nil)
	hooked.between = func() {
		hooked.between = nil
		if _, err := w.admin.Exec(context.Background(), `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, role, string(access.ContentShare)); err != nil {
			t.Error(err)
		}
	}
	in := domain.NewUser{Email: w.email("tt"), FullName: "Đặng Gia Hân", RoleID: role, Delivery: domain.DeliveryEmailLink}

	_, err := w.create(w.peopleOnly(), in)

	if !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
		t.Fatalf("error = %v, want the subset rule's refusal for a role that grew past the caller", err)
	}
	if hooked.reads != 2 {
		t.Errorf("the role was read %d times, want 2: before the write and once more after it changed", hooked.reads)
	}
	w.nothingWritten(t, in.Email)
}

func TestARoleRenamedAfterTheCheckIsGivenWithItsNewName(t *testing.T) {
	w := newUsersWorld(t, true)
	role := w.customRole(t, access.TeachingGrading)
	hooked := w.watchingRoles(nil)
	hooked.between = func() {
		hooked.between = nil
		if _, err := w.admin.Exec(context.Background(), `UPDATE app.roles SET name = name || ' mới', revision = revision + 1 WHERE id = $1::uuid`, role); err != nil {
			t.Error(err)
		}
	}
	in := domain.NewUser{Email: w.email("dt"), FullName: "Vũ Minh Tâm", RoleID: role, Delivery: domain.DeliveryEmailLink}

	result, err := w.create(w.peopleOnly(), in)

	if err != nil {
		t.Fatal(err)
	}
	if result.User.Role.ID != role || hooked.reads != 2 {
		t.Errorf("role = %s read %d times, want %s read twice", result.User.Role.ID, hooked.reads, role)
	}
	var recorded string
	if err := w.admin.QueryRow(context.Background(), `SELECT diff->>'role' FROM app.audit_log WHERE entity_id = $1::uuid`, result.User.ID).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != result.User.Role.Name {
		t.Errorf("audit names the role %q, want its current name %q", recorded, result.User.Role.Name)
	}
}

func TestARoleThatKeepsChangingIsNotGivenAndNothingIsWritten(t *testing.T) {
	w := newUsersWorld(t, true)
	role := w.customRole(t, access.TeachingGrading)
	hooked := w.watchingRoles(func() {
		if _, err := w.admin.Exec(context.Background(), `UPDATE app.roles SET revision = revision + 1 WHERE id = $1::uuid`, role); err != nil {
			t.Error(err)
		}
	})
	in := domain.NewUser{Email: w.email("lt"), FullName: "Phan Quốc Bảo", RoleID: role, Delivery: domain.DeliveryEmailLink}

	_, err := w.create(w.peopleOnly(), in)

	if !errors.Is(err, domain.ErrRoleChanged) {
		t.Fatalf("error = %v, want ErrRoleChanged", err)
	}
	if hooked.reads != 3 {
		t.Errorf("the role was read %d times, want 3 and no more", hooked.reads)
	}
	w.nothingWritten(t, in.Email)
}
