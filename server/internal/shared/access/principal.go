package access

// Builtin names one of the four built-in roles. A custom role has none.
type Builtin string

// The built-in roles, as app.roles.builtin_key stores them.
const (
	BuiltinAdmin     Builtin = "admin"
	BuiltinTeacher   Builtin = "teacher"
	BuiltinAssistant Builtin = "assistant"
	BuiltinStudent   Builtin = "student"
)

// Principal is who a request acts as, resolved from the user's role: its
// effective permissions, the session epoch a token must carry, and whether the
// account is disabled.
type Principal struct {
	UserID      string
	RoleID      string
	BuiltinKey  Builtin
	Permissions Set
	Epoch       int
	Disabled    bool
}

// Scope returns the rows the principal reaches: every teacher's when it holds
// scope.all, its own otherwise.
func (p Principal) Scope() Scope {
	return Scope{UserID: p.UserID, All: p.Permissions.Has(ScopeAll)}
}

// IsStudentLike reports whether a role is a strict student target: the
// built-in Student, or a custom role whose grants hold nothing but
// learning.take_tests. Every other built-in role is excluded, the Admin with
// "Take tests" turned on included. It is app.student_like_roles in Go.
func IsStudentLike(builtin Builtin, grants Set) bool {
	switch builtin {
	case BuiltinStudent:
		return true
	case "":
		return grants.Without(LearningTakeTests).Len() == 0
	}
	return false
}

// CanActOn reports whether an actor holding actor may act on a person holding
// target, under the subset rule (D13): every key of the target except
// learning.take_tests, which gives no power over anyone, must be the actor's.
// Both sets are effective permissions, the Admin's wildcard expanded.
func CanActOn(actor, target Set) bool {
	return target.Without(LearningTakeTests).SubsetOf(actor)
}
