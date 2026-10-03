package access

import (
	"slices"
	"strings"
)

// Requirement is the permission an operation declares in x-permission: one
// key, one pseudo-key, or a list meaning any of them.
type Requirement struct {
	anyOf []Key
}

// AnyOf returns the requirement that any one of keys meets.
func AnyOf(keys ...Key) Requirement { return Requirement{anyOf: slices.Clone(keys)} }

// Keys returns the requirement's keys in the order they were declared.
func (r Requirement) Keys() []Key { return slices.Clone(r.anyOf) }

// SatisfiedBy reports whether a principal holding s meets r. Self is met by
// any s, because a disabled or unknown user is refused before any requirement
// is checked; WorkspaceTeacher and WorkspaceAdmin are met when Workspaces
// opens that workspace. A requirement with no keys is never met.
func (r Requirement) SatisfiedBy(s Set) bool {
	for _, k := range r.anyOf {
		switch k {
		case Self:
			return true
		case WorkspaceTeacher:
			if opensTeacher(s) {
				return true
			}
		case WorkspaceAdmin:
			if opensAdmin(s) {
				return true
			}
		default:
			if s.Has(k) {
				return true
			}
		}
	}
	return false
}

// Workspace is a console a user may enter.
type Workspace string

// The three workspaces, in the order Workspaces lists them.
const (
	TeacherWorkspace Workspace = "teacher"
	AdminWorkspace   Workspace = "admin"
	AppWorkspace     Workspace = "app"
)

// Workspaces returns the consoles s opens, in the order teacher, admin, app:
// the teacher workspace for any content, teaching or people.students key; the
// Admin console for people.users.manage, people.roles.manage,
// system.audit.read, system.settings.write or scope.all; the student app for
// learning.take_tests.
func Workspaces(s Set) []Workspace {
	out := []Workspace{}
	if opensTeacher(s) {
		out = append(out, TeacherWorkspace)
	}
	if opensAdmin(s) {
		out = append(out, AdminWorkspace)
	}
	if s.Has(LearningTakeTests) {
		out = append(out, AppWorkspace)
	}
	return out
}

var teacherPrefixes = []string{"content.", "teaching.", "people.students."}

var adminKeys = []Key{PeopleUsersManage, PeopleRolesManage, SystemAuditRead, SystemSettingsWrite, ScopeAll}

func opensTeacher(s Set) bool {
	for k := range s.keys {
		for _, prefix := range teacherPrefixes {
			if strings.HasPrefix(string(k), prefix) {
				return true
			}
		}
	}
	return false
}

func opensAdmin(s Set) bool {
	return slices.ContainsFunc(adminKeys, s.Has)
}
