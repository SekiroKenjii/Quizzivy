package application_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/application/model"
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

func TestAnAssignableRoleSaysWhetherItOpensTheTeacherWorkspaceInBothQueries(t *testing.T) {
	app := queryApp(t)
	roles, err := app.Queries.AssignableRoles.Handle(context.Background(), query.AssignableRoles{By: adminCaller()})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range roles {
		got[r.Name] = r.OpensTeacher
		one, err := app.Queries.AssignableRole.Handle(context.Background(), query.AssignableRole{By: adminCaller(), RoleID: r.ID})
		if err != nil {
			t.Fatal(err)
		}
		if one.OpensTeacher != r.OpensTeacher {
			t.Errorf("%s: AssignableRole says %v, AssignableRoles %v", r.Name, one.OpensTeacher, r.OpensTeacher)
		}
	}
	want := map[string]bool{"Admin": true, "Teacher": true, "Custom": true, "Student": false, "Only tests": false}
	if !maps.Equal(got, want) {
		t.Errorf("opens the teacher workspace = %v, want %v", got, want)
	}
}

func TestOneAssignableRoleIsAnswerForEachRefusal(t *testing.T) {
	app := queryApp(t)
	grader := domain.Caller{RoleID: "role-custom", Permissions: access.NewSet(access.TeachingGrading)}
	cases := []struct {
		name    string
		by      domain.Caller
		roleID  string
		want    error
		student bool
	}{
		{"the Admin gives the Teacher role", adminCaller(), "role-teacher", nil, false},
		{"the Admin gives a student target", adminCaller(), "role-only-tests", nil, true},
		{"the Admin gives the built-in Student", adminCaller(), "role-student", nil, true},
		{"nobody gives the Assistant", adminCaller(), "role-assistant", domain.ErrNotAssignable, false},
		{"an unknown role", adminCaller(), "role-missing", domain.ErrRoleNotFound, false},
		{"a grader cannot give the Teacher role", grader, "role-teacher", domain.ErrTargetRoleNotAllowed, false},
		{"a grader gives a student target", grader, "role-only-tests", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			role, err := app.Queries.AssignableRole.Handle(context.Background(), query.AssignableRole{By: c.by, RoleID: c.roleID})
			if !errors.Is(err, c.want) || (c.want == nil && err != nil) {
				t.Fatalf("error = %v, want %v", err, c.want)
			}
			if c.want == nil && (role.ID != c.roleID || role.StudentTarget != c.student) {
				t.Errorf("role = %+v, want %s with studentTarget %v", role, c.roleID, c.student)
			}
			if c.want != nil && role != (model.AssignableRole{}) {
				t.Errorf("a refusal returned %+v", role)
			}
		})
	}
}

func TestEveryRoleIsListedWithTheCallersVerdictOnGivingIt(t *testing.T) {
	app := queryApp(t)
	grader := domain.Caller{RoleID: "role-custom", Permissions: access.NewSet(access.TeachingGrading)}
	type verdict struct {
		verdict model.RoleVerdict
		student bool
	}
	for name, tc := range map[string]struct {
		by   domain.Caller
		want map[string]verdict
	}{
		"the Admin": {adminCaller(), map[string]verdict{
			"Admin": {model.RoleGivable, false}, "Teacher": {model.RoleGivable, false}, "Assistant": {model.RoleNeverGiven, false},
			"Custom": {model.RoleGivable, false}, "Student": {model.RoleGivable, true}, "Only tests": {model.RoleGivable, true},
		}},
		"a grader": {grader, map[string]verdict{
			"Admin": {model.RoleBeyondCaller, false}, "Teacher": {model.RoleBeyondCaller, false}, "Assistant": {model.RoleNeverGiven, false},
			"Custom": {model.RoleGivable, false}, "Student": {model.RoleGivable, true}, "Only tests": {model.RoleGivable, true},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			choices, err := app.Queries.RoleChoices.Handle(context.Background(), query.RoleChoices{By: tc.by})
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]verdict{}
			for _, c := range choices {
				got[c.Name] = verdict{c.Verdict, c.StudentTarget}
			}
			if len(choices) != len(tc.want) {
				t.Errorf("%d roles listed, want %d", len(choices), len(tc.want))
			}
			for role, want := range tc.want {
				if got[role] != want {
					t.Errorf("%s: %+v, want %+v", role, got[role], want)
				}
			}
		})
	}
}

func TestTheVerdictOfRoleChoicesIsTheOneAssignableRoleReaches(t *testing.T) {
	app := queryApp(t)
	grader := domain.Caller{RoleID: "role-custom", Permissions: access.NewSet(access.TeachingGrading)}
	for _, by := range []domain.Caller{adminCaller(), grader} {
		choices, err := app.Queries.RoleChoices.Handle(context.Background(), query.RoleChoices{By: by})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range choices {
			_, err := app.Queries.AssignableRole.Handle(context.Background(), query.AssignableRole{By: by, RoleID: c.ID})
			want := map[model.RoleVerdict]error{model.RoleGivable: nil, model.RoleBeyondCaller: domain.ErrTargetRoleNotAllowed, model.RoleNeverGiven: domain.ErrNotAssignable}[c.Verdict]
			if !errors.Is(err, want) || (want == nil && err != nil) {
				t.Errorf("%s: AssignableRole says %v, RoleChoices %v", c.Name, err, c.Verdict)
			}
		}
	}
}
