package access_test

import (
	"slices"
	"testing"

	"quizzivy/internal/shared/access"
)

var (
	teacherGrants = access.NewSet(
		access.ContentTestsWrite, access.ContentTestsPublish, access.ContentQuestionsWrite,
		access.ContentMediaWrite, access.ContentShare,
		access.TeachingClassesWrite, access.TeachingAssignmentsWrite, access.TeachingGrading,
		access.TeachingAttemptsIntervene, access.TeachingAttendance,
		access.PeopleStudentsRead, access.PeopleStudentsCreate, access.PeopleStudentsResetPassword,
	)
	assistantGrants = access.NewSet(
		access.ContentTestsWrite, access.ContentQuestionsWrite, access.TeachingAssignmentsWrite,
		access.TeachingGrading, access.TeachingAttendance, access.PeopleStudentsRead,
	)
	studentGrants    = access.NewSet(access.LearningTakeTests)
	adminEffective   = access.NewSet(access.All()...).Without(access.LearningTakeTests)
	adminTakingTests = access.NewSet(access.All()...)
)

func TestTheCatalogueHoldsTwentyTwoDistinctKeysWithTheHiddenFourLast(t *testing.T) {
	all := access.All()
	if len(all) != 22 {
		t.Fatalf("len(All()) = %d, want 22", len(all))
	}
	if access.NewSet(all...).Len() != 22 {
		t.Fatal("All() repeats a key")
	}
	for i, k := range all {
		if !k.Known() || k.Pseudo() {
			t.Errorf("%q: Known() = %v, Pseudo() = %v", k, k.Known(), k.Pseudo())
		}
		if want := i >= 18; k.Hidden() != want {
			t.Errorf("%q at %d: Hidden() = %v, want %v", k, i, k.Hidden(), want)
		}
	}
	all[0] = "changed"
	if access.All()[0] != access.ContentTestsWrite {
		t.Error("All() returned the package's own slice")
	}
}

func TestThePseudoKeysAreNotCatalogueKeys(t *testing.T) {
	for _, k := range []access.Key{access.Self, access.WorkspaceTeacher, access.WorkspaceAdmin} {
		if k.Known() || !k.Pseudo() || k.Hidden() {
			t.Errorf("%q: Known() = %v, Pseudo() = %v, Hidden() = %v", k, k.Known(), k.Pseudo(), k.Hidden())
		}
	}
}

func TestSelfIsMetByAnySet(t *testing.T) {
	for name, s := range map[string]access.Set{"empty": {}, "student": studentGrants, "admin": adminEffective} {
		if !access.AnyOf(access.Self).SatisfiedBy(s) {
			t.Errorf("self not met by %s", name)
		}
	}
}

func TestTheTeacherWorkspaceOpensForTheAssistantButNotForAStudent(t *testing.T) {
	requirement := access.AnyOf(access.WorkspaceTeacher)
	if !requirement.SatisfiedBy(assistantGrants) {
		t.Error("workspace.teacher not met by the Assistant's six keys")
	}
	for _, k := range assistantGrants.Keys() {
		if !requirement.SatisfiedBy(access.NewSet(k)) {
			t.Errorf("workspace.teacher not met by %q alone", k)
		}
	}
	if requirement.SatisfiedBy(studentGrants) {
		t.Error("workspace.teacher met by {learning.take_tests}")
	}
	if requirement.SatisfiedBy(access.NewSet(access.PeopleUsersManage)) {
		t.Error("workspace.teacher met by people.users.manage, which opens only the Admin console")
	}
}

func TestTheAdminWorkspaceOpensForItsFiveKeysOnly(t *testing.T) {
	requirement := access.AnyOf(access.WorkspaceAdmin)
	for _, k := range []access.Key{
		access.PeopleUsersManage, access.PeopleRolesManage, access.SystemAuditRead,
		access.SystemSettingsWrite, access.ScopeAll,
	} {
		if !requirement.SatisfiedBy(access.NewSet(k)) {
			t.Errorf("workspace.admin not met by %q", k)
		}
	}
	for name, s := range map[string]access.Set{"teacher": teacherGrants, "assistant": assistantGrants, "student": studentGrants} {
		if requirement.SatisfiedBy(s) {
			t.Errorf("workspace.admin met by the %s", name)
		}
	}
}

func TestAListIsMetByAnyOfItsKeys(t *testing.T) {
	requirement := access.AnyOf(access.ContentQuestionsWrite, access.ContentTestsWrite)
	cases := []struct {
		set  access.Set
		want bool
	}{
		{access.NewSet(access.ContentQuestionsWrite), true},
		{access.NewSet(access.ContentTestsWrite), true},
		{access.NewSet(access.ContentMediaWrite), false},
		{access.Set{}, false},
	}
	for _, c := range cases {
		if got := requirement.SatisfiedBy(c.set); got != c.want {
			t.Errorf("SatisfiedBy(%v) = %v, want %v", c.set.Keys(), got, c.want)
		}
	}
	if access.AnyOf().SatisfiedBy(adminTakingTests) {
		t.Error("a requirement with no keys was met")
	}
}

func TestAKeyRequirementIsMetOnlyByThatKey(t *testing.T) {
	requirement := access.AnyOf(access.TeachingGrading)
	if !requirement.SatisfiedBy(assistantGrants) || requirement.SatisfiedBy(studentGrants) {
		t.Error("teaching.grading: want met by the Assistant and not by a student")
	}
}

func TestWorkspacesFollowTheRole(t *testing.T) {
	cases := []struct {
		name string
		set  access.Set
		want []access.Workspace
	}{
		{"admin", adminEffective, []access.Workspace{access.TeacherWorkspace, access.AdminWorkspace}},
		{"admin taking tests", adminTakingTests, []access.Workspace{access.TeacherWorkspace, access.AdminWorkspace, access.AppWorkspace}},
		{"teacher", teacherGrants, []access.Workspace{access.TeacherWorkspace}},
		{"assistant", assistantGrants, []access.Workspace{access.TeacherWorkspace}},
		{"student", studentGrants, []access.Workspace{access.AppWorkspace}},
		{"no grants", access.Set{}, []access.Workspace{}},
	}
	for _, c := range cases {
		if got := access.Workspaces(c.set); !slices.Equal(got, c.want) {
			t.Errorf("%s: Workspaces = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTheSubsetRule(t *testing.T) {
	usersManager := access.NewSet(access.PeopleUsersManage)
	cases := []struct {
		name          string
		actor, target access.Set
		want          bool
	}{
		{"admin on a student", adminEffective, studentGrants, true},
		{"admin on an admin taking tests", adminEffective, adminTakingTests, true},
		{"teacher on a student", teacherGrants, studentGrants, true},
		{"users manager on an admin taking tests", usersManager, adminTakingTests, false},
		{"users manager on a student", usersManager, studentGrants, true},
		{"teacher on an admin", teacherGrants, adminEffective, false},
		{"assistant on a teacher", assistantGrants, teacherGrants, false},
	}
	for _, c := range cases {
		if got := access.CanActOn(c.actor, c.target); got != c.want {
			t.Errorf("%s: CanActOn = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStudentLikeRoles(t *testing.T) {
	cases := []struct {
		name    string
		builtin access.Builtin
		grants  access.Set
		want    bool
	}{
		{"built-in Student", access.BuiltinStudent, studentGrants, true},
		{"admin with take tests granted", access.BuiltinAdmin, studentGrants, false},
		{"built-in Teacher", access.BuiltinTeacher, teacherGrants, false},
		{"built-in Assistant with no grants", access.BuiltinAssistant, access.Set{}, false},
		{"empty custom role", "", access.Set{}, true},
		{"custom role that only takes tests", "", studentGrants, true},
		{"custom role that also grades", "", access.NewSet(access.LearningTakeTests, access.TeachingGrading), false},
	}
	for _, c := range cases {
		if got := access.IsStudentLike(c.builtin, c.grants); got != c.want {
			t.Errorf("%s: IsStudentLike = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestASetIsImmutableAndListsInCatalogueOrder(t *testing.T) {
	s := access.NewSet(access.LearningTakeTests, "zeta.unknown", access.ContentShare, access.ContentTestsWrite, "alpha.unknown", access.ContentShare)
	want := []access.Key{access.ContentTestsWrite, access.ContentShare, access.LearningTakeTests, "alpha.unknown", "zeta.unknown"}
	if got := s.Keys(); !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
	smaller := s.Without(access.ContentShare, access.SystemLeads)
	if !s.Has(access.ContentShare) || smaller.Has(access.ContentShare) || smaller.Len() != 4 {
		t.Errorf("Without changed the original or missed a key: s = %v, smaller = %v", s.Keys(), smaller.Keys())
	}
	if !smaller.SubsetOf(s) || s.SubsetOf(smaller) {
		t.Error("SubsetOf disagrees with Without")
	}
	var zero access.Set
	if zero.Len() != 0 || zero.Has(access.Self) || !zero.SubsetOf(zero) || len(zero.Keys()) != 0 || zero.Without(access.ScopeAll).Len() != 0 {
		t.Error("the zero Set is not an empty set")
	}
}

func TestARequirementKeepsItsDeclarationOrderAndItsOwnCopy(t *testing.T) {
	keys := []access.Key{access.TeachingGrading, access.TeachingAttemptsIntervene}
	requirement := access.AnyOf(keys...)
	keys[0] = access.ScopeAll
	got := requirement.Keys()
	if !slices.Equal(got, []access.Key{access.TeachingGrading, access.TeachingAttemptsIntervene}) {
		t.Fatalf("Keys() = %v after the caller's slice changed", got)
	}
	got[1] = access.ScopeAll
	if requirement.Keys()[1] != access.TeachingAttemptsIntervene {
		t.Error("Keys() returned the requirement's own slice")
	}
}

func TestAPrincipalWithScopeAllReachesEveryRow(t *testing.T) {
	admin := access.Principal{UserID: "u1", Permissions: adminEffective}
	teacher := access.Principal{UserID: "u2", Permissions: teacherGrants}
	if got := admin.Scope(); got != (access.Scope{UserID: "u1", All: true}) {
		t.Errorf("admin scope = %+v", got)
	}
	if got := teacher.Scope(); got != (access.Scope{UserID: "u2", All: false}) {
		t.Errorf("teacher scope = %+v", got)
	}
}
