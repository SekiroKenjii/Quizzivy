package query

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
)

// Matrix asks for every role with its grants, as the Roles & permissions page
// shows them.
type Matrix struct{}

// MatrixHandler answers Matrix from the database.
type MatrixHandler struct {
	Repo domain.Repository
	*support.Service
}

// Handle returns the built-in roles in the order Admin, Teacher, Assistant,
// Student, then custom roles oldest first, each with its grants and its
// effective permissions.
func (h MatrixHandler) Handle(ctx context.Context, _ Matrix) ([]model.RoleGrants, error) {
	roles, err := h.Repo.Roles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.RoleGrants, 0, len(roles))
	for _, r := range roles {
		out = append(out, model.RoleGrants{
			RoleID:    r.ID,
			Builtin:   r.Builtin,
			Name:      r.Name,
			Revision:  r.Revision,
			Grants:    r.Grants,
			Effective: h.Effective(r.Role, r.Grants),
		})
	}
	return out, nil
}
