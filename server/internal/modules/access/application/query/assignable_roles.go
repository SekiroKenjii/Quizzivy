package query

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

// AssignableRoles asks for the roles By may give to a user.
type AssignableRoles struct {
	By domain.Caller
}

// AssignableRolesHandler answers AssignableRoles from the database.
type AssignableRolesHandler struct {
	Repo domain.Repository
	*support.Service
}

// Handle returns the roles whose permissions, learning.take_tests apart, are a
// subset of the caller's, without the Assistant, in the matrix's order.
func (h AssignableRolesHandler) Handle(ctx context.Context, q AssignableRoles) ([]model.AssignableRole, error) {
	roles, err := h.Repo.Roles(ctx)
	if err != nil {
		return nil, err
	}
	allowed := h.Guard().Assignable(q.By, roles)
	out := make([]model.AssignableRole, 0, len(allowed))
	for _, r := range allowed {
		out = append(out, model.AssignableRole{
			ID: r.ID, Builtin: r.Builtin, Name: r.Name, Icon: r.Icon, Color: r.Color,
			StudentTarget: access.IsStudentLike(r.Builtin, r.Grants),
			OpensTeacher:  opensTeacher(h.Guard().Effective(r)),
		})
	}
	return out, nil
}
