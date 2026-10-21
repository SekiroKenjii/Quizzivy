package query

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

// ListRoles asks for every role and the matrix rows, as By sees them.
type ListRoles struct {
	By domain.Caller
}

// ListRolesHandler answers ListRoles from the database.
type ListRolesHandler struct {
	Repo domain.Repository
	*support.Service
}

// Handle returns the roles in the order Admin, Teacher, Assistant, Student,
// then custom roles oldest first, and the catalogue without its hidden keys.
func (h ListRolesHandler) Handle(ctx context.Context, q ListRoles) (model.RoleList, error) {
	roles, err := h.Repo.Roles(ctx)
	if err != nil {
		return model.RoleList{}, err
	}
	permissions, err := h.Repo.Permissions(ctx)
	if err != nil {
		return model.RoleList{}, err
	}
	heads, err := h.Repo.Headcounts(ctx)
	if err != nil {
		return model.RoleList{}, err
	}
	out := model.RoleList{Roles: make([]model.Role, 0, len(roles))}
	for _, r := range roles {
		out.Roles = append(out.Roles, h.RoleView(q.By, r, heads[r.ID]))
	}
	for _, p := range permissions {
		if p.InMatrix {
			out.Catalogue = append(out.Catalogue, model.CatalogueEntry{Key: p.Key, Group: p.Group, AdminLocked: p.Key != access.LearningTakeTests})
		}
	}
	return out, nil
}
