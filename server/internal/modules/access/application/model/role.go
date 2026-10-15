package model

import "quizzivy/internal/shared/access"

// Role is a role as the Roles & permissions page shows it to one caller.
// Permissions are the role's effective permissions in catalogue order without
// the hidden keys. Editable is false where the subset rule keeps the caller
// from the role.
type Role struct {
	ID                 string
	Builtin            access.Builtin
	Name               string
	Description        string
	Icon               string
	Color              string
	UserCount          int
	Permissions        []access.Key
	Revision           int64
	Editable           bool
	InProgressAttempts int
}

// Custom reports whether the role is not built in.
func (r Role) Custom() bool { return r.Builtin == "" }

// CatalogueEntry is one row of the matrix.
type CatalogueEntry struct {
	Key         access.Key
	Group       string
	AdminLocked bool
}

// RoleList is every role and the matrix rows they are drawn against.
type RoleList struct {
	Roles     []Role
	Catalogue []CatalogueEntry
}

// AssignableRole is a role a caller may give to a user.
type AssignableRole struct {
	ID            string
	Builtin       access.Builtin
	Name          string
	Icon          string
	Color         string
	StudentTarget bool
}

// RoleVerdict is what a caller may do about giving a role to a user.
type RoleVerdict int

const (
	// RoleGivable means the caller may give the role.
	RoleGivable RoleVerdict = iota
	// RoleBeyondCaller means the role holds a permission the caller lacks.
	RoleBeyondCaller
	// RoleNeverGiven means the role is the Assistant's, which no one gives.
	RoleNeverGiven
)

// RoleChoice is a role with the caller's verdict on giving it.
type RoleChoice struct {
	ID            string
	Builtin       access.Builtin
	Name          string
	StudentTarget bool
	Verdict       RoleVerdict
}
