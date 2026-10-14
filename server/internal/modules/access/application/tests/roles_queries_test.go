package application_test

import (
	"context"
	"slices"
	"testing"

	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

func queryApp(t *testing.T) *application.Application {
	t.Helper()
	repo := newRepo()
	repo.roles = []domain.RoleRow{rowAdmin, rowTeacher, rowAssistant, rowCustom, {
		Role: domain.Role{ID: "role-student", Builtin: access.BuiltinStudent, Revision: 1}, Name: "Student", Grants: access.NewSet(access.LearningTakeTests),
	}, {
		Role: domain.Role{ID: "role-only-tests", Revision: 1}, Name: "Only tests", Grants: access.NewSet(access.LearningTakeTests),
	}}
	groups := map[string]string{"content": "content", "teaching": "teaching", "people": "people", "system": "system"}
	for _, k := range access.All() {
		group := "system"
		for prefix, g := range groups {
			if len(k) > len(prefix) && string(k)[:len(prefix)+1] == prefix+"." {
				group = g
			}
		}
		repo.permissions = append(repo.permissions, domain.PermissionRow{Key: k, Group: group, InMatrix: !k.Hidden()})
	}
	repo.heads = map[string]domain.Headcount{"role-teacher": {Users: 4, InProgressAttempts: 2}}
	return application.New(repo)
}

func TestTheRoleListCarriesTheCatalogueAndTheHeadcounts(t *testing.T) {
	app := queryApp(t)
	grader := domain.Caller{RoleID: "role-custom", Permissions: access.NewSet(access.TeachingGrading, access.PeopleRolesManage)}
	list, err := app.Queries.ListRoles.Handle(context.Background(), query.ListRoles{By: grader})
	if err != nil {
		t.Fatal(err)
	}
	var matrix []access.Key
	for _, e := range list.Catalogue {
		matrix = append(matrix, e.Key)
		if want := e.Key != access.LearningTakeTests; e.AdminLocked != want {
			t.Errorf("%s adminLocked = %v, want %v", e.Key, e.AdminLocked, want)
		}
	}
	want := slices.DeleteFunc(access.All(), access.Key.Hidden)
	if !slices.Equal(matrix, want) {
		t.Errorf("catalogue = %v, want the keys minus the hidden ones", matrix)
	}
	byName := map[string]int{}
	for i, r := range list.Roles {
		byName[r.Name] = i
	}
	teacher := list.Roles[byName["Teacher"]]
	if teacher.UserCount != 4 || teacher.InProgressAttempts != 2 || teacher.Editable || teacher.Custom() {
		t.Errorf("Teacher = %+v, want 4 users, 2 attempts, out of the grader's reach and built in", teacher)
	}
	custom := list.Roles[byName["Custom"]]
	if !custom.Editable || !custom.Custom() || custom.Revision != 3 || !slices.Equal(custom.Permissions, []access.Key{access.TeachingGrading}) {
		t.Errorf("Custom = %+v", custom)
	}
	admin := list.Roles[byName["Admin"]]
	if admin.Editable || slices.ContainsFunc(admin.Permissions, access.Key.Hidden) || slices.Contains(admin.Permissions, access.LearningTakeTests) {
		t.Errorf("Admin = %+v, want out of reach, with no hidden key and no Take tests", admin)
	}
}

func TestTheAssignableRolesAreThoseWithinTheCallersReach(t *testing.T) {
	app := queryApp(t)
	roles, err := app.Queries.AssignableRoles.Handle(context.Background(), query.AssignableRoles{By: adminCaller()})
	if err != nil {
		t.Fatal(err)
	}
	type pick struct {
		name    string
		student bool
	}
	var got []pick
	for _, r := range roles {
		got = append(got, pick{r.Name, r.StudentTarget})
	}
	want := []pick{{"Admin", false}, {"Teacher", false}, {"Custom", false}, {"Student", true}, {"Only tests", true}}
	if !slices.Equal(got, want) {
		t.Errorf("the Admin may assign %v, want %v", got, want)
	}
}
