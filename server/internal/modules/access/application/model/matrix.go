// Package model holds the access read models the application returns.
package model

import "quizzivy/internal/shared/access"

// RoleGrants is one column of the Roles & permissions matrix: a role, its
// stored grants and the permissions its users hold.
type RoleGrants struct {
	RoleID    string
	Builtin   access.Builtin
	Name      string
	Revision  int64
	Grants    access.Set
	Effective access.Set
}
