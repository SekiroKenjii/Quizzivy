package query

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

// RolePermissions asks what a role's users may do, at the role's revision.
type RolePermissions struct {
	Role domain.Role
}

// RolePermissionsHandler answers RolePermissions from grants cached by
// (role, revision).
type RolePermissionsHandler struct {
	*support.Service
}

// Handle returns the role's effective permissions.
func (h RolePermissionsHandler) Handle(ctx context.Context, q RolePermissions) (access.Set, error) {
	grants, err := h.RoleGrants(ctx, q.Role)
	if err != nil {
		return access.Set{}, err
	}
	return h.Effective(q.Role, grants), nil
}
