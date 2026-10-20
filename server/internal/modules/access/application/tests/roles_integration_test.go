//go:build integration

package application_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/command"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type realWorld struct {
	t    *testing.T
	pool *pgxpool.Pool
	tx   pgx.Tx
	app  *application.Application
	tag  string
}

func appWorld(t *testing.T) *realWorld {
	t.Helper()
	return openWorld(t, pgx.TxOptions{})
}

func snapshotWorld(t *testing.T) *realWorld {
	t.Helper()
	return openWorld(t, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
}

func openWorld(t *testing.T, opts pgx.TxOptions) *realWorld {
	t.Helper()
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback: %v", err)
		}
	})
	var role string
	if err := tx.QueryRow(ctx, `SELECT current_user`).Scan(&role); err != nil || role != "quizzivy_app" {
		t.Fatalf("connected as %q (%v), want the app role", role, err)
	}
	repo := repositories.NewPostgres(db.NewContext(tx))
	return &realWorld{t: t, pool: pool, tx: tx, app: application.New(repo).WithRoleStore(repo), tag: uuid.NewString()[:8]}
}

func (w *realWorld) one(sql string, args ...any) string {
	w.t.Helper()
	var out string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&out); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return out
}

func (w *realWorld) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.tx.Exec(context.Background(), sql, args...); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
}

func (w *realWorld) builtin(key access.Builtin) string {
	return w.one(`SELECT id::text FROM app.roles WHERE builtin_key = $1`, string(key))
}

func (w *realWorld) role(name string, keys ...access.Key) string {
	w.t.Helper()
	id := w.one(`INSERT INTO app.roles (name, icon, color) VALUES ($1, 'star', 'amber') RETURNING id::text`, name+" "+w.tag)
	for _, k := range keys {
		w.exec(`INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, $2)`, id, string(k))
	}
	return id
}

func (w *realWorld) user(roleID string) string {
	w.t.Helper()
	return w.one(`INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Role fixture', $2) RETURNING id::text`, uuid.NewString()+"@example.com", roleID)
}

func (w *realWorld) caller(userID string) domain.Caller {
	w.t.Helper()
	p, err := w.app.Queries.ResolvePrincipal.Handle(context.Background(), query.ResolvePrincipal{UserID: userID})
	if err != nil {
		w.t.Fatal(err)
	}
	return domain.Caller{Actor: actor.Actor{ID: userID, IP: "203.0.113.9", UserAgent: "role test"}, RoleID: p.RoleID, Permissions: p.Permissions}
}

func (w *realWorld) admin() domain.Caller { return w.caller(w.user(w.builtin(access.BuiltinAdmin))) }

func (w *realWorld) adminWithoutTakeTests() domain.Caller {
	w.t.Helper()
	w.exec(`DELETE FROM app.role_permissions WHERE role_id = $1 AND permission_key = $2`, w.builtin(access.BuiltinAdmin), string(access.LearningTakeTests))
	return w.admin()
}

func (w *realWorld) revision(roleID string) int64 {
	w.t.Helper()
	var r int64
	if err := w.tx.QueryRow(context.Background(), `SELECT revision FROM app.roles WHERE id = $1`, roleID).Scan(&r); err != nil {
		w.t.Fatal(err)
	}
	return r
}

func (w *realWorld) audits(action, roleID string) int {
	w.t.Helper()
	var n int
	if err := w.tx.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE action = $1 AND entity = 'role' AND entity_id = $2`, action, roleID).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *realWorld) roleOf(by domain.Caller, id string) model.Role {
	w.t.Helper()
	list, err := w.app.Queries.ListRoles.Handle(context.Background(), query.ListRoles{By: by})
	if err != nil {
		w.t.Fatal(err)
	}
	for _, r := range list.Roles {
		if r.ID == id {
			return r
		}
	}
	w.t.Fatalf("role %s is not listed", id)
	return model.Role{}
}

func save(w *realWorld, by domain.Caller, revisions map[string]int64, changes ...domain.GrantChange) ([]model.Role, error) {
	var expected []command.RoleRevision
	for id, r := range revisions {
		expected = append(expected, command.RoleRevision{RoleID: id, Revision: r})
	}
	return w.app.Commands.SetRolePermissions.Handle(context.Background(), command.SetRolePermissions{By: by, Changes: changes, Expected: expected})
}

func TestTheAppRoleCannotDeleteARoleOrWriteThePermissionCatalogue(t *testing.T) {
	w := appWorld(t)
	for _, statement := range []string{
		`DELETE FROM app.roles WHERE false`,
		`UPDATE app.roles SET builtin_key = NULL WHERE false`,
		`INSERT INTO app.permissions (key, group_key, ordinal, in_matrix) VALUES ('made.up', 'system', 99, true)`,
		`UPDATE app.role_permissions SET role_id = role_id WHERE false`,
	} {
		_, err := w.pool.Exec(context.Background(), statement)
		var refused *pgconn.PgError
		if !errors.As(err, &refused) || refused.Code != "42501" {
			t.Errorf("%s: %v, want permission denied", statement, err)
		}
	}
}

func TestAnAdminCopiesTheTeacherIntoACustomRole(t *testing.T) {
	w := appWorld(t)
	by := w.admin()
	name := "Tổ chấm " + w.tag
	created, err := w.app.Commands.CreateRole.Handle(context.Background(), command.CreateRole{By: by, Name: name, Description: "Chấm bài", CopyFrom: w.builtin(access.BuiltinTeacher)})
	if err != nil {
		t.Fatal(err)
	}
	teacher := w.roleOf(by, w.builtin(access.BuiltinTeacher))
	if !slices.Equal(created.Permissions, teacher.Permissions) || len(created.Permissions) == 0 {
		t.Errorf("the copy holds %v, the Teacher %v", created.Permissions, teacher.Permissions)
	}
	if !created.Custom() || created.Icon != "key-round" || created.Color != "gray" || created.Name != name || !created.Editable {
		t.Errorf("created = %+v", created)
	}
	if w.audits(domain.AuditRoleCreated, created.ID) != 1 {
		t.Error("no role.created audit row")
	}
	if w.one(`SELECT copied_from::text FROM app.roles WHERE id = $1`, created.ID) != w.builtin(access.BuiltinTeacher) {
		t.Error("copied_from does not name the Teacher")
	}
	_, err = w.app.Commands.CreateRole.Handle(context.Background(), command.CreateRole{By: by, Name: strings.ToUpper(name)})
	if !errors.Is(err, domain.ErrNameTaken) {
		t.Errorf("a name in other letters: %v, want ErrNameTaken", err)
	}
}

func TestACopyFromTheAssistantIsRefusedAndWritesNothing(t *testing.T) {
	w := appWorld(t)
	_, err := w.app.Commands.CreateRole.Handle(context.Background(), command.CreateRole{By: w.admin(), Name: "Từ trợ giảng " + w.tag, CopyFrom: w.builtin(access.BuiltinAssistant)})
	if !errors.Is(err, domain.ErrNotAssignable) {
		t.Fatalf("error = %v, want ErrNotAssignable", err)
	}
	if n := w.one(`SELECT count(*)::text FROM app.roles WHERE name = $1`, "Từ trợ giảng "+w.tag); n != "0" {
		t.Errorf("%s roles were left behind", n)
	}
}

func TestAMembersNextRequestSeesTheNewSet(t *testing.T) {
	w := appWorld(t)
	role := w.role("Thành viên", access.TeachingGrading)
	member := w.user(role)
	by := w.admin()
	before, err := w.app.Resolve(context.Background(), member)
	if err != nil || before.Permissions.Has(access.ContentMediaWrite) {
		t.Fatalf("before the save: %v %v", before.Permissions.Keys(), err)
	}
	roles, err := save(w, by, map[string]int64{role: w.revision(role)}, domain.GrantChange{RoleID: role, Key: access.ContentMediaWrite, Granted: true})
	if err != nil {
		t.Fatal(err)
	}
	after, err := w.app.Resolve(context.Background(), member)
	if err != nil || !after.Permissions.Has(access.ContentMediaWrite) || !after.Permissions.Has(access.TeachingGrading) {
		t.Fatalf("after the save: %v %v", after.Permissions.Keys(), err)
	}
	if roles[0].Revision != w.revision(role) || roles[0].Revision <= 1 {
		t.Errorf("answered revision %d, stored %d", roles[0].Revision, w.revision(role))
	}
	if w.audits(domain.AuditRolePermissionsChanged, role) != 1 {
		t.Error("no role.permissions_changed audit row")
	}
	if _, err := save(w, by, map[string]int64{role: w.revision(role)}, domain.GrantChange{RoleID: role, Key: access.TeachingGrading, Granted: false}); err != nil {
		t.Fatal(err)
	}
	if revoked, _ := w.app.Resolve(context.Background(), member); revoked.Permissions.Has(access.TeachingGrading) {
		t.Error("a revoked key is still held on the next request")
	}
}

func TestAStaleSaveWritesNothingForAnyRole(t *testing.T) {
	w := appWorld(t)
	first, second := w.role("Một", access.TeachingGrading), w.role("Hai", access.TeachingGrading)
	by := w.admin()
	startFirst, startSecond := w.revision(first), w.revision(second)
	_, err := save(w, by, map[string]int64{first: startFirst, second: startSecond - 1},
		domain.GrantChange{RoleID: first, Key: access.ContentMediaWrite, Granted: true},
		domain.GrantChange{RoleID: second, Key: access.ContentMediaWrite, Granted: true})
	if !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("error = %v, want ErrStaleWrite", err)
	}
	if w.revision(first) != startFirst || w.revision(second) != startSecond {
		t.Error("a refused save moved a revision")
	}
	if n := w.one(`SELECT count(*)::text FROM app.role_permissions WHERE permission_key = 'content.media.write' AND role_id = ANY($1::uuid[])`, []string{first, second}); n != "0" {
		t.Errorf("%s grants written by a refused save", n)
	}
}

func TestAnAdminWithoutTakeTestsGrantsItToACustomRole(t *testing.T) {
	w := appWorld(t)
	by := w.adminWithoutTakeTests()
	role := w.role("Chỉ làm bài")
	roles, err := save(w, by, map[string]int64{role: w.revision(role)}, domain.GrantChange{RoleID: role, Key: access.LearningTakeTests, Granted: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(roles[0].Permissions, []access.Key{access.LearningTakeTests}) {
		t.Errorf("the role holds %v", roles[0].Permissions)
	}
}

func TestAnAdminTurnsItsOwnRolesTakeTestsOnAndOff(t *testing.T) {
	w := appWorld(t)
	adminRole := w.builtin(access.BuiltinAdmin)
	by := w.adminWithoutTakeTests()
	if _, err := save(w, by, map[string]int64{adminRole: w.revision(adminRole)}, domain.GrantChange{RoleID: adminRole, Key: access.LearningTakeTests, Granted: true}); err != nil {
		t.Fatalf("turning it on: %v", err)
	}
	if on, _ := w.app.Resolve(context.Background(), by.ID); !on.Permissions.Has(access.LearningTakeTests) {
		t.Error("the Admin does not hold Take tests after turning it on")
	}
	if _, err := save(w, by, map[string]int64{adminRole: w.revision(adminRole)}, domain.GrantChange{RoleID: adminRole, Key: access.LearningTakeTests, Granted: false}); err != nil {
		t.Fatalf("turning it off: %v", err)
	}
	if off, _ := w.app.Resolve(context.Background(), by.ID); off.Permissions.Has(access.LearningTakeTests) {
		t.Error("the Admin holds Take tests after turning it off")
	}
	_, err := save(w, by, map[string]int64{adminRole: w.revision(adminRole)}, domain.GrantChange{RoleID: adminRole, Key: access.ContentShare, Granted: false})
	if !errors.Is(err, domain.ErrRoleLocked) {
		t.Errorf("another Admin cell: %v, want ErrRoleLocked", err)
	}
}

func TestARoleManagerStaysInsideItsOwnPermissions(t *testing.T) {
	w := appWorld(t)
	managerRole := w.role("Quản lý vai trò", access.PeopleRolesManage, access.TeachingGrading, access.ContentTestsWrite)
	manager := w.caller(w.user(managerRole))
	within := w.role("Trong tầm", access.TeachingGrading)
	teacher := w.builtin(access.BuiltinTeacher)
	ctx := context.Background()

	if _, err := save(w, manager, map[string]int64{within: w.revision(within)}, domain.GrantChange{RoleID: within, Key: access.ContentTestsWrite, Granted: true}); err != nil {
		t.Errorf("granting a key the manager holds to a role within reach: %v", err)
	}
	for _, c := range []struct {
		name   string
		role   string
		key    access.Key
		grant  bool
		wanted error
	}{
		{"a key the manager lacks", within, access.ContentShare, true, domain.ErrTargetRoleNotAllowed},
		{"a role beyond the manager", teacher, access.ContentTestsWrite, true, domain.ErrTargetRoleNotAllowed},
		{"the manager's own role", managerRole, access.TeachingGrading, false, domain.ErrCannotActOnSelf},
		{"a hidden key", within, access.ScopeAll, true, domain.ErrKeyNotGrantable},
	} {
		if _, err := save(w, manager, map[string]int64{c.role: w.revision(c.role)}, domain.GrantChange{RoleID: c.role, Key: c.key, Granted: c.grant}); !errors.Is(err, c.wanted) {
			t.Errorf("%s: %v, want %v", c.name, err, c.wanted)
		}
	}
	renamed := "Đổi tên " + w.tag
	if _, err := w.app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: manager, ID: teacher, Expected: w.revision(teacher), Name: &renamed}); !errors.Is(err, domain.ErrTargetRoleNotAllowed) {
		t.Errorf("renaming a role beyond the manager: %v", err)
	}
	if _, err := w.app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: manager, Name: "Sao chép " + w.tag, CopyFrom: teacher}); !errors.Is(err, domain.ErrTargetRoleNotAllowed) {
		t.Errorf("copying a role beyond the manager: %v", err)
	}
	roles, err := w.app.Queries.AssignableRoles.Handle(ctx, query.AssignableRoles{By: manager})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range roles {
		if r.Builtin == access.BuiltinAdmin || r.Builtin == access.BuiltinTeacher || r.Builtin == access.BuiltinAssistant {
			t.Errorf("the manager may assign the %s role", r.Builtin)
		}
	}
}

func TestEditingARoleKeepsItsKeyAndMovesItsRevision(t *testing.T) {
	w := appWorld(t)
	by := w.admin()
	teacher := w.builtin(access.BuiltinTeacher)
	start := w.revision(teacher)
	name, icon := "Giáo viên "+w.tag, "crown"
	updated, err := w.app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: by, ID: teacher, Expected: start, Name: &name, Icon: &icon})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Builtin != access.BuiltinTeacher || updated.Name != name || updated.Icon != "crown" || updated.Revision != start+1 {
		t.Errorf("updated = %+v, want the Teacher renamed at revision %d", updated, start+1)
	}
	if w.audits(domain.AuditRoleUpdated, teacher) != 1 {
		t.Error("no role.updated audit row")
	}
	if _, err := w.app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: by, ID: teacher, Expected: start, Name: &name}); !errors.Is(err, domain.ErrStaleWrite) {
		t.Errorf("a save at the old revision: %v, want ErrStaleWrite", err)
	}
	other := w.role("Khác")
	taken := strings.ToLower(name)
	if _, err := w.app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: by, ID: other, Expected: w.revision(other), Name: &taken}); !errors.Is(err, domain.ErrNameTaken) {
		t.Errorf("a name taken in other letters: %v, want ErrNameTaken", err)
	}
	same := updated.Revision
	if _, err := w.app.Commands.UpdateRole.Handle(context.Background(), command.UpdateRole{By: by, ID: teacher, Expected: same, Name: &name}); err != nil || w.revision(teacher) != same {
		t.Errorf("an unchanged edit: %v, revision %d, want %d", err, w.revision(teacher), same)
	}
}

func TestTheRoleListCountsTheCatalogueAndTheHolders(t *testing.T) {
	w := appWorld(t)
	by := w.admin()
	role := w.role("Có người")
	w.user(role)
	w.user(role)
	list, err := w.app.Queries.ListRoles.Handle(context.Background(), query.ListRoles{By: by})
	if err != nil {
		t.Fatal(err)
	}
	var counted model.Role
	for _, r := range list.Roles {
		if r.ID == role {
			counted = r
		}
	}
	if counted.UserCount != 2 || counted.InProgressAttempts != 0 || !counted.Editable {
		t.Errorf("role = %+v, want two holders", counted)
	}
	if first := list.Roles[0]; first.Builtin != access.BuiltinAdmin || list.Roles[1].Builtin != access.BuiltinTeacher || list.Roles[2].Builtin != access.BuiltinAssistant || list.Roles[3].Builtin != access.BuiltinStudent {
		t.Errorf("the roles start %v %v %v %v", first.Builtin, list.Roles[1].Builtin, list.Roles[2].Builtin, list.Roles[3].Builtin)
	}
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	var contract []access.Key
	for _, v := range spec.Components.Schemas["PermissionKey"].Value.Enum {
		if k := access.Key(v.(string)); !k.Hidden() {
			contract = append(contract, k)
		}
	}
	var catalogue []access.Key
	for _, e := range list.Catalogue {
		catalogue = append(catalogue, e.Key)
	}
	if !slices.Equal(catalogue, contract) {
		t.Errorf("catalogue %v differs from the contract's PermissionKey enum minus the hidden keys %v", catalogue, contract)
	}
}

func TestADecomposedNameCollidesWithTheComposedOneAndIsStoredComposed(t *testing.T) {
	w := appWorld(t)
	by, ctx := w.admin(), context.Background()
	builtin := w.roleOf(by, w.builtin(access.BuiltinTeacher)).Name
	decomposed := norm.NFD.String(builtin)
	if decomposed == builtin {
		t.Fatalf("the Teacher's name %q has no decomposed form", builtin)
	}
	if _, err := w.app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: decomposed}); !errors.Is(err, domain.ErrNameTaken) {
		t.Errorf("the Teacher's name decomposed: %v, want ErrNameTaken", err)
	}
	fresh := "Việt Nam " + w.tag
	created, err := w.app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: norm.NFD.String(fresh)})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != fresh {
		t.Errorf("stored %q, want the composed %q", created.Name, fresh)
	}
	other := w.role("Khác")
	name := norm.NFD.String(fresh)
	_, err = w.app.Commands.UpdateRole.Handle(ctx, command.UpdateRole{By: by, ID: other, Expected: w.revision(other), Name: &name})
	if !errors.Is(err, domain.ErrNameTaken) {
		t.Errorf("a rename to the decomposed name of another role: %v, want ErrNameTaken", err)
	}
}

func TestNoRoleIsCreatedPastTheLimit(t *testing.T) {
	w := snapshotWorld(t)
	by, ctx := w.admin(), context.Background()
	w.exec(`
		INSERT INTO app.roles (name, icon, color)
		SELECT 'Fill ' || n || ' ' || $1::text, 'star', 'amber'
		  FROM generate_series(1, $2::int - (SELECT count(*) FROM app.roles)::int - 1) AS n`, w.tag, domain.MaxRoles)
	if got := w.one(`SELECT count(*)::text FROM app.roles`); got != strconv.Itoa(domain.MaxRoles-1) {
		t.Fatalf("%s roles after the fill, want %d", got, domain.MaxRoles-1)
	}
	if _, err := w.app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Cuối " + w.tag}); err != nil {
		t.Fatalf("the last role that fits: %v", err)
	}
	_, err := w.app.Commands.CreateRole.Handle(ctx, command.CreateRole{By: by, Name: "Thừa " + w.tag})
	if !errors.Is(err, domain.ErrRoleLimit) {
		t.Fatalf("one role too many: %v, want ErrRoleLimit", err)
	}
	if got := w.one(`SELECT count(*)::text FROM app.roles`); got != strconv.Itoa(domain.MaxRoles) {
		t.Errorf("%s roles, want %d", got, domain.MaxRoles)
	}
	list, err := w.app.Queries.ListRoles.Handle(ctx, query.ListRoles{By: by})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Roles) != domain.MaxRoles {
		t.Errorf("the list holds %d roles, the contract allows %d", len(list.Roles), domain.MaxRoles)
	}
}
