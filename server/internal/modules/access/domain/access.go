// Package domain is the access model: roles and the grants they store, the
// PermissionManager that expands a role's grants into the permissions its
// users hold, and the principal row a request is resolved from.
package domain

import (
	"context"
	"errors"

	"quizzivy/internal/shared/access"
)

// ErrUnknownUser means no user has the id a principal was asked for.
var ErrUnknownUser = errors.New("access: unknown user")

// Role is a role as the access model needs it: its id, its built-in key if it
// has one, and the revision its grants are at. The revision moves with every
// grant change, so grants cached under (ID, Revision) are never stale.
type Role struct {
	ID       string
	Builtin  access.Builtin
	Revision int64
}

// PrincipalRow is what one read returns about a user: the role, whether the
// account is disabled, and the session epoch its tokens must carry.
type PrincipalRow struct {
	Role     Role
	Disabled bool
	Epoch    int
}

// RoleRow is a role with what the Roles & permissions matrix shows of it: its
// name and its stored grants.
type RoleRow struct {
	Role
	Name   string
	Grants access.Set
}

// Repository is the read side of the access model.
type Repository interface {
	// Principal reads a user's role, disabled state and session epoch in one
	// statement, or returns ErrUnknownUser.
	Principal(ctx context.Context, userID string) (PrincipalRow, error)
	// Grants returns the keys a role stores.
	Grants(ctx context.Context, roleID string) (access.Set, error)
	// Catalogue returns app.permissions' keys in matrix order.
	Catalogue(ctx context.Context) ([]access.Key, error)
	// Roles returns every role with its grants: the built-in roles in the
	// order Admin, Teacher, Assistant, Student, then custom roles oldest first.
	Roles(ctx context.Context) ([]RoleRow, error)
}

// PermissionManager expands a role's stored grants into the permissions its
// users hold.
type PermissionManager struct{}

// Effective returns what a role may do. The Admin holds every key of catalogue
// except learning.take_tests, and that key too when it is granted, so a key a
// later release compiles in reaches the Admin with no data change. Every other
// role holds exactly its grants.
func (PermissionManager) Effective(role Role, grants access.Set, catalogue []access.Key) access.Set {
	if role.Builtin != access.BuiltinAdmin {
		return grants
	}
	keys := make([]access.Key, 0, len(catalogue))
	for _, k := range catalogue {
		if k != access.LearningTakeTests || grants.Has(access.LearningTakeTests) {
			keys = append(keys, k)
		}
	}
	return access.NewSet(keys...)
}
