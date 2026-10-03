//go:build integration

package repositories_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type world struct {
	pool   *pgxpool.Pool
	repo   *repositories.Postgres
	roleID string
	userID string
}

func newWorld(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	w := world{pool: pool, repo: repositories.NewPostgres(db.NewContext(pool))}
	tag := uuid.NewString()
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.roles (name, icon, color) VALUES ($1, 'star', 'amber') RETURNING id::text`,
		"Access fixture "+tag).Scan(&w.roleID); err != nil {
		pool.Close()
		t.Fatalf("role: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1`, w.userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM app.roles WHERE id = $1`, w.roleID); err != nil {
			t.Errorf("cleanup role: %v", err)
		}
		pool.Close()
	})
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Access fixture', $2) RETURNING id::text`,
		tag+"@example.com", w.roleID).Scan(&w.userID); err != nil {
		t.Fatalf("user: %v", err)
	}
	return w
}

func (w world) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func resolve(t *testing.T, app *application.Application, userID string) access.Principal {
	t.Helper()
	p, err := app.Queries.ResolvePrincipal.Handle(context.Background(), query.ResolvePrincipal{UserID: userID})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return p
}

func TestADisabledUserResolvesAsDisabled(t *testing.T) {
	w := newWorld(t)
	app := application.New(w.repo)
	if resolve(t, app, w.userID).Disabled {
		t.Fatal("an active user resolved as disabled")
	}
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1`, w.userID)
	app.Forget(w.userID)
	if !resolve(t, app, w.userID).Disabled {
		t.Error("a disabled user resolved as active")
	}
}

func TestTheSessionEpochIsRead(t *testing.T) {
	w := newWorld(t)
	w.exec(t, `UPDATE app.users SET session_epoch = 5 WHERE id = $1`, w.userID)
	if got := resolve(t, application.New(w.repo), w.userID); got.Epoch != 5 || got.RoleID != w.roleID || got.BuiltinKey != "" {
		t.Errorf("principal = %+v, want epoch 5 on the fixture role", got)
	}
}

func TestAGrantBumpsTheRevisionAndReachesThePrincipalAfterForgetAll(t *testing.T) {
	w := newWorld(t)
	app := application.New(w.repo)
	if got := resolve(t, app, w.userID); got.Permissions.Len() != 0 {
		t.Fatalf("a role with no grants holds %v", got.Permissions.Keys())
	}
	var before, after int64
	if err := w.pool.QueryRow(context.Background(), `SELECT revision FROM app.roles WHERE id = $1`, w.roleID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	w.exec(t, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'teaching.grading')`, w.roleID)
	if err := w.pool.QueryRow(context.Background(), `SELECT revision FROM app.roles WHERE id = $1`, w.roleID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("revision %d after a grant, want %d", after, before+1)
	}
	if got := resolve(t, app, w.userID); got.Permissions.Len() != 0 {
		t.Fatalf("a cached principal changed before ForgetAll: %v", got.Permissions.Keys())
	}
	app.ForgetAll()
	if got := resolve(t, app, w.userID); !slices.Equal(got.Permissions.Keys(), []access.Key{access.TeachingGrading}) {
		t.Errorf("after ForgetAll: %v, want [teaching.grading]", got.Permissions.Keys())
	}
}

func TestTheCatalogueAndTheRolesAreReadInMatrixOrder(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	catalogue, err := w.repo.Catalogue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(catalogue, access.All()) {
		t.Errorf("Catalogue() = %v, want access.All()", catalogue)
	}
	roles, err := w.repo.Roles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) < 5 {
		t.Fatalf("%d roles, want the four built-in ones and the fixture", len(roles))
	}
	var builtins []access.Builtin
	for _, r := range roles[:4] {
		builtins = append(builtins, r.Builtin)
	}
	if want := []access.Builtin{access.BuiltinAdmin, access.BuiltinTeacher, access.BuiltinAssistant, access.BuiltinStudent}; !slices.Equal(builtins, want) {
		t.Errorf("the first four roles are %v, want %v", builtins, want)
	}
	found := false
	for _, r := range roles[4:] {
		if r.Builtin != "" {
			t.Errorf("built-in role %s listed after the custom roles began", r.Builtin)
		}
		found = found || r.ID == w.roleID
	}
	if !found {
		t.Error("the fixture role is not listed after the built-in roles")
	}
}
