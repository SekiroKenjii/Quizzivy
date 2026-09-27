package domain_test

import (
	"slices"
	"testing"

	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

func TestTheAdminHoldsEveryCompiledKeyButTakeTests(t *testing.T) {
	admin := domain.Role{ID: "r1", Builtin: access.BuiltinAdmin, Revision: 1}
	got := domain.PermissionManager{}.Effective(admin, access.Set{}, access.All())
	want := access.NewSet(access.All()...).Without(access.LearningTakeTests)
	if !slices.Equal(got.Keys(), want.Keys()) {
		t.Errorf("Effective(admin) = %v, want %v", got.Keys(), want.Keys())
	}
}

func TestTheAdminTakesTestsWhenThatCellIsOn(t *testing.T) {
	admin := domain.Role{ID: "r1", Builtin: access.BuiltinAdmin, Revision: 2}
	got := domain.PermissionManager{}.Effective(admin, access.NewSet(access.LearningTakeTests), access.All())
	if !slices.Equal(got.Keys(), access.All()) {
		t.Errorf("Effective(admin taking tests) = %v, want every key", got.Keys())
	}
}

func TestAKeyALaterReleaseCompilesInReachesTheAdminWithNoDataChange(t *testing.T) {
	admin := domain.Role{ID: "r1", Builtin: access.BuiltinAdmin, Revision: 1}
	later := append(access.All(), "teaching.sessions.write")
	if got := (domain.PermissionManager{}).Effective(admin, access.Set{}, later); !got.Has("teaching.sessions.write") {
		t.Errorf("Effective(admin) = %v, missing the new key", got.Keys())
	}
}

func TestEveryOtherRoleHoldsExactlyItsGrants(t *testing.T) {
	grants := access.NewSet(access.TeachingGrading, access.PeopleStudentsRead)
	for _, builtin := range []access.Builtin{access.BuiltinTeacher, access.BuiltinAssistant, access.BuiltinStudent, ""} {
		role := domain.Role{ID: "r", Builtin: builtin, Revision: 1}
		if got := (domain.PermissionManager{}).Effective(role, grants, access.All()); !slices.Equal(got.Keys(), grants.Keys()) {
			t.Errorf("Effective(%q) = %v, want %v", builtin, got.Keys(), grants.Keys())
		}
	}
}
