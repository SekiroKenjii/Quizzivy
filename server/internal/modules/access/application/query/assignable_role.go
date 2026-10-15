package query

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

// AssignableRole asks whether By may give one role to a user, and what the role is.
type AssignableRole struct {
	By     domain.Caller
	RoleID string
}

// AssignableRoleHandler answers AssignableRole from the database.
type AssignableRoleHandler struct {
	Repo domain.Repository
	*support.Service
}

// Handle returns the role, with the same fields AssignableRoles lists. It fails
// with domain.ErrRoleNotFound when no role has the id, domain.ErrNotAssignable
// for the Assistant and domain.ErrTargetRoleNotAllowed when the subset rule
// keeps the caller from the role.
func (h AssignableRoleHandler) Handle(ctx context.Context, q AssignableRole) (model.AssignableRole, error) {
	roles, err := h.Repo.Roles(ctx)
	if err != nil {
		return model.AssignableRole{}, err
	}
	for _, r := range roles {
		if r.ID != q.RoleID {
			continue
		}
		if err := h.Guard().CheckSource(q.By, r); err != nil {
			return model.AssignableRole{}, err
		}
		return model.AssignableRole{
			ID: r.ID, Builtin: r.Builtin, Name: r.Name, Icon: r.Icon, Color: r.Color,
			StudentTarget: access.IsStudentLike(r.Builtin, r.Grants), Revision: r.Revision,
		}, nil
	}
	return model.AssignableRole{}, domain.ErrRoleNotFound
}
