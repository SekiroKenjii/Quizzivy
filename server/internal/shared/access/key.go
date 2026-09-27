// Package access is the kernel's model of authorization: the permission
// catalogue as Keys, a role's grants as a Set, the Requirement an operation
// declares in x-permission, the subset rule between an actor and the person
// it acts on, and the Principal and Scope a request carries.
package access

import "slices"

// Key is one permission in the catalogue, or a pseudo-key that a Requirement
// evaluates against a Set.
type Key string

// The catalogue, in the order of the Roles & permissions matrix. The last four
// are hidden: no role is granted them, and only the Admin wildcard holds them.
const (
	ContentTestsWrite           Key = "content.tests.write"
	ContentTestsPublish         Key = "content.tests.publish"
	ContentQuestionsWrite       Key = "content.questions.write"
	ContentMediaWrite           Key = "content.media.write"
	ContentShare                Key = "content.share"
	TeachingClassesWrite        Key = "teaching.classes.write"
	TeachingAssignmentsWrite    Key = "teaching.assignments.write"
	TeachingGrading             Key = "teaching.grading"
	TeachingAttemptsIntervene   Key = "teaching.attempts.intervene"
	TeachingAttendance          Key = "teaching.attendance"
	PeopleStudentsRead          Key = "people.students.read"
	PeopleStudentsCreate        Key = "people.students.create"
	PeopleStudentsResetPassword Key = "people.students.reset_password"
	PeopleUsersManage           Key = "people.users.manage"
	PeopleRolesManage           Key = "people.roles.manage"
	SystemAuditRead             Key = "system.audit.read"
	SystemSettingsWrite         Key = "system.settings.write"
	LearningTakeTests           Key = "learning.take_tests"
	ScopeAll                    Key = "scope.all"
	SystemAPIReference          Key = "system.api_reference"
	SystemDataExport            Key = "system.data_export"
	SystemLeads                 Key = "system.leads"
)

// The pseudo-keys, which no role stores and a Requirement evaluates: Self is
// met by any signed-in user, WorkspaceTeacher by any key that opens the
// teacher workspace, and WorkspaceAdmin by any key that opens the Admin
// console.
const (
	Self             Key = "self"
	WorkspaceTeacher Key = "workspace.teacher"
	WorkspaceAdmin   Key = "workspace.admin"
)

var catalogue = []Key{
	ContentTestsWrite, ContentTestsPublish, ContentQuestionsWrite, ContentMediaWrite, ContentShare,
	TeachingClassesWrite, TeachingAssignmentsWrite, TeachingGrading, TeachingAttemptsIntervene, TeachingAttendance,
	PeopleStudentsRead, PeopleStudentsCreate, PeopleStudentsResetPassword, PeopleUsersManage, PeopleRolesManage,
	SystemAuditRead, SystemSettingsWrite, LearningTakeTests,
	ScopeAll, SystemAPIReference, SystemDataExport, SystemLeads,
}

var position = func() map[Key]int {
	m := make(map[Key]int, len(catalogue))
	for i, k := range catalogue {
		m[k] = i
	}
	return m
}()

var hidden = map[Key]bool{ScopeAll: true, SystemAPIReference: true, SystemDataExport: true, SystemLeads: true}

var pseudo = map[Key]bool{Self: true, WorkspaceTeacher: true, WorkspaceAdmin: true}

// All returns the catalogue's keys in matrix order, the hidden keys last. The
// caller may modify the slice.
func All() []Key { return slices.Clone(catalogue) }

// Known reports whether k is a catalogue key. A pseudo-key is not.
func (k Key) Known() bool {
	_, ok := position[k]
	return ok
}

// Hidden reports whether k is a catalogue key that no matrix row shows and no
// role may be granted.
func (k Key) Hidden() bool { return hidden[k] }

// Pseudo reports whether k is one of the pseudo-keys.
func (k Key) Pseudo() bool { return pseudo[k] }
