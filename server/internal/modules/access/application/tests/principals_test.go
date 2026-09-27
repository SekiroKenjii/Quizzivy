package application_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

type fakeRepo struct {
	mu         sync.Mutex
	users      map[string]domain.PrincipalRow
	grants     map[string]access.Set
	catalogue  []access.Key
	roles      []domain.RoleRow
	userReads  int
	grantReads int
	duringRead func()
}

func (f *fakeRepo) Principal(_ context.Context, userID string) (domain.PrincipalRow, error) {
	f.mu.Lock()
	f.userReads++
	row, ok := f.users[userID]
	hook := f.duringRead
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if !ok {
		return domain.PrincipalRow{}, domain.ErrUnknownUser
	}
	return row, nil
}

func (f *fakeRepo) Grants(_ context.Context, roleID string) (access.Set, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grantReads++
	return f.grants[roleID], nil
}

func (f *fakeRepo) Catalogue(context.Context) ([]access.Key, error) { return f.catalogue, nil }

func (f *fakeRepo) Roles(context.Context) ([]domain.RoleRow, error) { return f.roles, nil }

func (f *fakeRepo) reads() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.userReads, f.grantReads
}

func (f *fakeRepo) setUser(id string, row domain.PrincipalRow) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[id] = row
}

var (
	adminRole   = domain.Role{ID: "role-admin", Builtin: access.BuiltinAdmin, Revision: 1}
	teacherRole = domain.Role{ID: "role-teacher", Builtin: access.BuiltinTeacher, Revision: 1}
	studentRole = domain.Role{ID: "role-student", Builtin: access.BuiltinStudent, Revision: 1}
)

func newRepo() *fakeRepo {
	return &fakeRepo{
		users: map[string]domain.PrincipalRow{
			"owner":   {Role: adminRole},
			"teacher": {Role: teacherRole, Epoch: 3},
			"lan":     {Role: studentRole},
			"minh":    {Role: studentRole, Disabled: true},
		},
		grants: map[string]access.Set{
			"role-teacher": access.NewSet(access.TeachingGrading, access.ContentTestsWrite),
			"role-student": access.NewSet(access.LearningTakeTests),
		},
		catalogue: access.All(),
	}
}

type clock struct{ now time.Time }

func (c *clock) read() time.Time     { return c.now }
func (c *clock) add(d time.Duration) { c.now = c.now.Add(d) }
func newClock() *clock               { return &clock{now: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)} }
func resolve(t *testing.T, app *application.Application, userID string) access.Principal {
	t.Helper()
	p, err := app.Queries.ResolvePrincipal.Handle(context.Background(), query.ResolvePrincipal{UserID: userID})
	if err != nil {
		t.Fatalf("resolve %s: %v", userID, err)
	}
	return p
}

func TestAPrincipalCarriesItsRoleEpochAndDisabledState(t *testing.T) {
	app := application.New(newRepo())
	owner := resolve(t, app, "owner")
	if owner.RoleID != "role-admin" || owner.BuiltinKey != access.BuiltinAdmin || owner.Permissions.Has(access.LearningTakeTests) || !owner.Permissions.Has(access.ScopeAll) {
		t.Errorf("owner = %+v, permissions %v", owner, owner.Permissions.Keys())
	}
	teacher := resolve(t, app, "teacher")
	if teacher.Epoch != 3 || !slices.Equal(teacher.Permissions.Keys(), []access.Key{access.ContentTestsWrite, access.TeachingGrading}) {
		t.Errorf("teacher = %+v, permissions %v", teacher, teacher.Permissions.Keys())
	}
	if minh := resolve(t, app, "minh"); !minh.Disabled {
		t.Error("a disabled student resolved as enabled")
	}
}

func TestAMissCostsOneUserReadAndOneGrantsReadPerRoleRevision(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	resolve(t, app, "lan")
	if users, grants := repo.reads(); users != 1 || grants != 1 {
		t.Fatalf("first miss: %d user reads, %d grants reads; want 1 and 1", users, grants)
	}
	resolve(t, app, "lan")
	if users, grants := repo.reads(); users != 1 || grants != 1 {
		t.Fatalf("a hit read the database: %d user reads, %d grants reads", users, grants)
	}
	repo.setUser("an", domain.PrincipalRow{Role: studentRole})
	resolve(t, app, "an")
	if users, grants := repo.reads(); users != 2 || grants != 1 {
		t.Fatalf("a second student: %d user reads, %d grants reads; want 2 and 1", users, grants)
	}
	moved := studentRole
	moved.Revision = 2
	repo.setUser("an", domain.PrincipalRow{Role: moved})
	app.Forget("an")
	resolve(t, app, "an")
	if users, grants := repo.reads(); users != 3 || grants != 2 {
		t.Fatalf("a new revision: %d user reads, %d grants reads; want 3 and 2", users, grants)
	}
}

func TestAPrincipalIsReadAgainAfterTenSeconds(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	c := newClock()
	app.SetClock(c.read)
	resolve(t, app, "teacher")
	c.add(9 * time.Second)
	resolve(t, app, "teacher")
	if users, _ := repo.reads(); users != 1 {
		t.Fatalf("within the TTL: %d user reads, want 1", users)
	}
	repo.setUser("teacher", domain.PrincipalRow{Role: teacherRole, Epoch: 4})
	c.add(time.Second)
	if got := resolve(t, app, "teacher"); got.Epoch != 4 {
		t.Errorf("after the TTL: epoch %d, want the re-read 4", got.Epoch)
	}
}

func TestForgetMakesTheNextRequestReadTheUser(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	resolve(t, app, "lan")
	repo.setUser("lan", domain.PrincipalRow{Role: studentRole, Disabled: true})
	if resolve(t, app, "lan").Disabled {
		t.Fatal("a cached principal changed without a Forget")
	}
	app.Forget("lan")
	if !resolve(t, app, "lan").Disabled {
		t.Error("after Forget the principal was not read again")
	}
}

func TestForgetAllDropsPrincipalsAndGrants(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	resolve(t, app, "lan")
	resolve(t, app, "teacher")
	app.ForgetAll()
	if app.Cached() != 0 {
		t.Fatalf("%d principals cached after ForgetAll", app.Cached())
	}
	resolve(t, app, "lan")
	if users, grants := repo.reads(); users != 3 || grants != 3 {
		t.Errorf("after ForgetAll: %d user reads, %d grants reads; want 3 and 3", users, grants)
	}
}

func TestAResolveRacingAForgetDoesNotCacheWhatItRead(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	repo.duringRead = func() { app.Forget("lan") }
	resolve(t, app, "lan")
	repo.duringRead = nil
	if app.Cached() != 0 {
		t.Fatal("a principal read before a Forget was cached")
	}
}

func TestAResolveRacingAForgetAllDoesNotCacheWhatItRead(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	repo.duringRead = func() { app.ForgetAll() }
	resolve(t, app, "teacher")
	repo.duringRead = nil
	if app.Cached() != 0 {
		t.Fatal("a principal read before a ForgetAll was cached")
	}
	before, _ := repo.reads()
	resolve(t, app, "teacher")
	if after, _ := repo.reads(); after != before+1 {
		t.Errorf("the next resolve did not read the user: %d reads, want %d", after, before+1)
	}
}

func TestTheCacheKeepsItsBoundDroppingTheLeastRecentlyUsed(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	app.SetLimit(2)
	resolve(t, app, "owner")
	resolve(t, app, "teacher")
	resolve(t, app, "owner")
	resolve(t, app, "lan")
	if app.Cached() != 2 {
		t.Fatalf("%d principals cached, want the bound 2", app.Cached())
	}
	before, _ := repo.reads()
	resolve(t, app, "owner")
	resolve(t, app, "lan")
	if after, _ := repo.reads(); after != before {
		t.Errorf("the two most recent users were read again: %d reads, want %d", after, before)
	}
	resolve(t, app, "teacher")
	if after, _ := repo.reads(); after != before+1 {
		t.Errorf("the least recently used user was not evicted: %d reads, want %d", after, before+1)
	}
}

func TestAnUnknownUserIsRefusedAndNotCached(t *testing.T) {
	repo := newRepo()
	app := application.New(repo)
	for range 2 {
		if _, err := app.Queries.ResolvePrincipal.Handle(context.Background(), query.ResolvePrincipal{UserID: "nobody"}); !errors.Is(err, domain.ErrUnknownUser) {
			t.Fatalf("err = %v, want ErrUnknownUser", err)
		}
	}
	if users, _ := repo.reads(); users != 2 {
		t.Errorf("%d user reads for two unknown lookups, want 2", users)
	}
}

func TestStartupRefusesACatalogueMissingACompiledKey(t *testing.T) {
	repo := newRepo()
	repo.catalogue = slices.DeleteFunc(access.All(), func(k access.Key) bool { return k == access.ContentShare })
	err := application.New(repo).CheckCatalogue(context.Background())
	if !errors.Is(err, application.ErrCatalogueBehind) {
		t.Fatalf("err = %v, want ErrCatalogueBehind", err)
	}
	repo.catalogue = append(access.All(), "teaching.sessions.write")
	if err := application.New(repo).CheckCatalogue(context.Background()); err != nil {
		t.Errorf("an extra row from a newer migration refused: %v", err)
	}
}

func TestTheMatrixCarriesGrantsAndEffectivePermissions(t *testing.T) {
	repo := newRepo()
	repo.roles = []domain.RoleRow{
		{Role: adminRole, Name: "Quản trị viên", Grants: access.Set{}},
		{Role: studentRole, Name: "Học viên", Grants: access.NewSet(access.LearningTakeTests)},
	}
	matrix, err := application.New(repo).Queries.Matrix.Handle(context.Background(), query.Matrix{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matrix) != 2 || matrix[0].Name != "Quản trị viên" || matrix[0].Grants.Len() != 0 || !matrix[0].Effective.Has(access.ScopeAll) {
		t.Fatalf("matrix = %+v", matrix)
	}
	if !slices.Equal(matrix[1].Effective.Keys(), []access.Key{access.LearningTakeTests}) {
		t.Errorf("student effective = %v", matrix[1].Effective.Keys())
	}
}

func TestRolePermissionsExpandsTheAdmin(t *testing.T) {
	app := application.New(newRepo())
	got, err := app.Queries.RolePermissions.Handle(context.Background(), query.RolePermissions{Role: adminRole})
	if err != nil {
		t.Fatal(err)
	}
	if got.Len() != len(access.All())-1 || got.Has(access.LearningTakeTests) {
		t.Errorf("admin permissions = %v", got.Keys())
	}
}
