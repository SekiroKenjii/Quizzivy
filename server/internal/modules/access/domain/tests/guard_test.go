package domain_test

import (
	"errors"
	"slices"
	"testing"

	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

var (
	adminRole     = domain.RoleRow{Role: domain.Role{ID: "admin", Builtin: access.BuiltinAdmin, Revision: 3}, Name: "Admin"}
	teacherRole   = domain.RoleRow{Role: domain.Role{ID: "teacher", Builtin: access.BuiltinTeacher, Revision: 4}, Name: "Teacher", Grants: access.NewSet(access.ContentTestsWrite, access.TeachingGrading, access.PeopleStudentsRead)}
	assistantRole = domain.RoleRow{Role: domain.Role{ID: "assistant", Builtin: access.BuiltinAssistant, Revision: 5}, Name: "Assistant", Grants: access.NewSet(access.TeachingGrading)}
	studentRole   = domain.RoleRow{Role: domain.Role{ID: "student", Builtin: access.BuiltinStudent, Revision: 6}, Name: "Student", Grants: access.NewSet(access.LearningTakeTests)}
	customRole    = domain.RoleRow{Role: domain.Role{ID: "custom", Revision: 7}, Name: "Custom", Grants: access.NewSet(access.TeachingGrading)}
	guard         = domain.NewGuard(access.All())
)

func callerHolding(roleID string, keys ...access.Key) domain.Caller {
	return domain.Caller{RoleID: roleID, Permissions: access.NewSet(keys...)}
}

func admin() domain.Caller {
	return domain.Caller{RoleID: "admin", Permissions: guard.Effective(adminRole)}
}

func roleMap(roles ...domain.RoleRow) map[string]domain.RoleRow {
	out := map[string]domain.RoleRow{}
	for _, r := range roles {
		out[r.ID] = r
	}
	return out
}

func revisions(roles ...domain.RoleRow) map[string]int64 {
	out := map[string]int64{}
	for _, r := range roles {
		out[r.ID] = r.Revision
	}
	return out
}

func TestTheVisiblePermissionsLeaveOutTheHiddenKeys(t *testing.T) {
	got := guard.Visible(adminRole)
	if slices.ContainsFunc(got, access.Key.Hidden) {
		t.Errorf("the Admin column shows a hidden key: %v", got)
	}
	if slices.Contains(got, access.LearningTakeTests) {
		t.Error("the Admin column shows Take tests while that cell is off")
	}
	if want := len(access.All()) - 4 - 1; len(got) != want {
		t.Errorf("the Admin column holds %d keys, want %d", len(got), want)
	}
	taking := adminRole
	taking.Grants = access.NewSet(access.LearningTakeTests)
	if !slices.Contains(guard.Visible(taking), access.LearningTakeTests) {
		t.Error("the Admin column omits Take tests while that cell is on")
	}
}

func TestACallerActsOnlyOnARoleWhosePermissionsAreItsOwn(t *testing.T) {
	grader := callerHolding("grader", access.TeachingGrading, access.PeopleRolesManage)
	for _, c := range []struct {
		role domain.RoleRow
		want bool
	}{
		{customRole, true},
		{assistantRole, true},
		{studentRole, true},
		{teacherRole, false},
		{adminRole, false},
	} {
		if got := guard.CanActOn(grader, c.role); got != c.want {
			t.Errorf("CanActOn(grader, %s) = %v, want %v", c.role.Name, got, c.want)
		}
	}
	if !guard.CanActOn(admin(), adminRole) || !guard.CanActOn(admin(), teacherRole) {
		t.Error("the Admin is stopped from a role")
	}
}

func TestTheAssignableRolesLeaveOutTheAssistantAndWhatTheCallerLacks(t *testing.T) {
	all := []domain.RoleRow{adminRole, teacherRole, assistantRole, studentRole, customRole}
	names := func(roles []domain.RoleRow) []string {
		var out []string
		for _, r := range roles {
			out = append(out, r.Name)
		}
		return out
	}
	if got, want := names(guard.Assignable(admin(), all)), []string{"Admin", "Teacher", "Student", "Custom"}; !slices.Equal(got, want) {
		t.Errorf("the Admin may assign %v, want %v", got, want)
	}
	grader := callerHolding("grader", access.TeachingGrading, access.ContentTestsWrite, access.PeopleStudentsRead)
	if got, want := names(guard.Assignable(grader, all)), []string{"Teacher", "Student", "Custom"}; !slices.Equal(got, want) {
		t.Errorf("a grader may assign %v, want %v", got, want)
	}
}

func TestACopySourceIsRefusedWhenItIsTheAssistantOrBeyondTheCaller(t *testing.T) {
	grader := callerHolding("grader", access.TeachingGrading)
	for _, c := range []struct {
		name   string
		by     domain.Caller
		source domain.RoleRow
		want   error
	}{
		{"the Assistant", admin(), assistantRole, domain.ErrNotAssignable},
		{"a role with more keys", grader, teacherRole, domain.ErrTargetRoleNotAllowed},
		{"a role within reach", grader, customRole, nil},
		{"the Student, whose one key gives no power", grader, studentRole, nil},
		{"the Teacher, for an Admin", admin(), teacherRole, nil},
	} {
		if err := guard.CheckSource(c.by, c.source); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestACopyCarriesTheVisiblePermissionsOfItsSource(t *testing.T) {
	if got, want := guard.Copy(teacherRole), teacherRole.Grants.Keys(); !slices.Equal(got, want) {
		t.Errorf("a copy of the Teacher starts with %v, want %v", got, want)
	}
	if slices.ContainsFunc(guard.Copy(adminRole), access.Key.Hidden) {
		t.Error("a copy of the Admin starts with a hidden key")
	}
}

func TestAnEditNeedsTheSubsetRuleAndTheRevisionTheCallerRead(t *testing.T) {
	grader := callerHolding("grader", access.TeachingGrading)
	if err := guard.CheckEdit(grader, teacherRole, teacherRole.Revision); !errors.Is(err, domain.ErrTargetRoleNotAllowed) {
		t.Errorf("an edit of a role beyond the caller: %v, want ErrTargetRoleNotAllowed", err)
	}
	if err := guard.CheckEdit(grader, customRole, customRole.Revision-1); !errors.Is(err, domain.ErrStaleWrite) {
		t.Errorf("an edit at an old revision: %v, want ErrStaleWrite", err)
	}
	if err := guard.CheckEdit(grader, customRole, customRole.Revision); err != nil {
		t.Errorf("a current edit of a role within reach: %v", err)
	}
	if err := guard.CheckEdit(admin(), adminRole, adminRole.Revision); err != nil {
		t.Errorf("the Admin editing the Admin role's text: %v", err)
	}
}

func TestEachGrantGuardRefusesItsOwnBreach(t *testing.T) {
	granter := callerHolding("own", access.PeopleRolesManage, access.TeachingGrading, access.ContentTestsWrite)
	own := domain.RoleRow{Role: domain.Role{ID: "own", Revision: 1}, Name: "Own", Grants: access.NewSet(access.PeopleRolesManage, access.TeachingGrading)}
	for _, c := range []struct {
		name   string
		by     domain.Caller
		role   domain.RoleRow
		change domain.GrantChange
		want   error
	}{
		{"an unknown role", granter, domain.RoleRow{Role: domain.Role{ID: "elsewhere"}}, domain.GrantChange{RoleID: "custom", Key: access.TeachingGrading, Granted: false}, domain.ErrRoleNotFound},
		{"a hidden key", admin(), customRole, domain.GrantChange{RoleID: "custom", Key: access.ScopeAll, Granted: true}, domain.ErrKeyNotGrantable},
		{"a key outside the catalogue", admin(), customRole, domain.GrantChange{RoleID: "custom", Key: "made.up", Granted: true}, domain.ErrKeyNotGrantable},
		{"an Admin cell but Take tests", admin(), adminRole, domain.GrantChange{RoleID: "admin", Key: access.TeachingGrading, Granted: true}, domain.ErrRoleLocked},
		{"the Student losing Take tests", admin(), studentRole, domain.GrantChange{RoleID: "student", Key: access.LearningTakeTests, Granted: false}, domain.ErrStudentNeedsTakeTests},
		{"the caller's own role", granter, own, domain.GrantChange{RoleID: "own", Key: access.ContentTestsWrite, Granted: true}, domain.ErrCannotActOnSelf},
		{"a role beyond the caller", granter, teacherRole, domain.GrantChange{RoleID: "teacher", Key: access.ContentTestsWrite, Granted: true}, domain.ErrTargetRoleNotAllowed},
		{"a key the caller lacks", granter, customRole, domain.GrantChange{RoleID: "custom", Key: access.ContentShare, Granted: true}, domain.ErrTargetRoleNotAllowed},
		{"a key the caller lacks, revoked", granter, domain.RoleRow{Role: customRole.Role, Name: "Custom", Grants: access.NewSet(access.TeachingGrading, access.ContentShare)}, domain.GrantChange{RoleID: "custom", Key: access.ContentShare, Granted: false}, domain.ErrTargetRoleNotAllowed},
	} {
		roles := roleMap(c.role)
		if c.change.RoleID != c.role.ID && c.want != domain.ErrRoleNotFound {
			t.Fatalf("%s: the change names %q, the role is %q", c.name, c.change.RoleID, c.role.ID)
		}
		_, err := guard.CheckGrants(c.by, roles, []domain.GrantChange{c.change}, revisions(c.role))
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestTheAdminMayTurnItsOwnTakeTestsOnAndOff(t *testing.T) {
	for _, granted := range []bool{true, false} {
		held := adminRole
		if !granted {
			held.Grants = access.NewSet(access.LearningTakeTests)
		}
		delta, err := guard.CheckGrants(admin(), roleMap(held), []domain.GrantChange{{RoleID: "admin", Key: access.LearningTakeTests, Granted: granted}}, revisions(held))
		if err != nil {
			t.Fatalf("the Admin setting its own Take tests to %v: %v", granted, err)
		}
		if got := delta["admin"]; got.Empty() {
			t.Errorf("the Admin setting its own Take tests to %v changed nothing", granted)
		}
	}
}

func TestAnAdminWithoutTakeTestsMayGrantItToACustomRole(t *testing.T) {
	delta, err := guard.CheckGrants(admin(), roleMap(customRole), []domain.GrantChange{{RoleID: "custom", Key: access.LearningTakeTests, Granted: true}}, revisions(customRole))
	if err != nil {
		t.Fatal(err)
	}
	if got := delta["custom"].Granted; !slices.Equal(got, []access.Key{access.LearningTakeTests}) {
		t.Errorf("granted %v, want Take tests", got)
	}
}

func TestACallerWithoutTakeTestsMayToggleItOnARoleWithinReach(t *testing.T) {
	grader := callerHolding("grader", access.PeopleRolesManage, access.TeachingGrading)
	if _, err := guard.CheckGrants(grader, roleMap(customRole), []domain.GrantChange{{RoleID: "custom", Key: access.LearningTakeTests, Granted: true}}, revisions(customRole)); err != nil {
		t.Errorf("a role manager without Take tests granting it: %v", err)
	}
	if _, err := guard.CheckGrants(grader, roleMap(adminRole), []domain.GrantChange{{RoleID: "admin", Key: access.LearningTakeTests, Granted: true}}, revisions(adminRole)); !errors.Is(err, domain.ErrTargetRoleNotAllowed) {
		t.Errorf("a role manager turning on the Admin's Take tests: %v, want ErrTargetRoleNotAllowed", err)
	}
}

func TestAStaleOrUnnamedRevisionRefusesTheWholeSave(t *testing.T) {
	changes := []domain.GrantChange{
		{RoleID: "custom", Key: access.TeachingGrading, Granted: false},
		{RoleID: "teacher", Key: access.ContentTestsWrite, Granted: false},
	}
	roles := roleMap(customRole, teacherRole)
	for name, expected := range map[string]map[string]int64{
		"a stale revision": {"custom": customRole.Revision, "teacher": teacherRole.Revision - 1},
		"no revision":      {"custom": customRole.Revision},
	} {
		if _, err := guard.CheckGrants(admin(), roles, changes, expected); !errors.Is(err, domain.ErrStaleWrite) {
			t.Errorf("%s: %v, want ErrStaleWrite", name, err)
		}
	}
	if _, err := guard.CheckGrants(admin(), roles, changes, revisions(customRole, teacherRole)); err != nil {
		t.Errorf("current revisions: %v", err)
	}
}

func TestACellLeftAsItWasChangesNothing(t *testing.T) {
	changes := []domain.GrantChange{
		{RoleID: "custom", Key: access.TeachingGrading, Granted: true},
		{RoleID: "custom", Key: access.ContentTestsWrite, Granted: false},
		{RoleID: "custom", Key: access.ContentMediaWrite, Granted: true},
	}
	delta, err := guard.CheckGrants(admin(), roleMap(customRole), changes, revisions(customRole))
	if err != nil {
		t.Fatal(err)
	}
	got := delta["custom"]
	if !slices.Equal(got.Granted, []access.Key{access.ContentMediaWrite}) || len(got.Revoked) != 0 {
		t.Errorf("delta = %+v, want only content.media.write granted", got)
	}
	if (domain.Delta{}).Empty() == false || got.Empty() {
		t.Error("Empty misreports a delta")
	}
}
