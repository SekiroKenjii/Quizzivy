package query

import (
	"context"
	"errors"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

// RoleChoices asks for every role with whether By may give it to a user.
type RoleChoices struct {
	By domain.Caller
}

// RoleChoicesHandler answers RoleChoices from the database.
type RoleChoicesHandler struct {
	Repo domain.Repository
	*support.Service
}

// Handle returns every role in the matrix's order, each with the same verdict
// AssignableRole reaches for it.
func (h RoleChoicesHandler) Handle(ctx context.Context, q RoleChoices) ([]model.RoleChoice, error) {
	roles, err := h.Repo.Roles(ctx)
	if err != nil {
		return nil, err
	}
	guard := h.Guard()
	out := make([]model.RoleChoice, 0, len(roles))
	for _, r := range roles {
		choice := model.RoleChoice{ID: r.ID, Builtin: r.Builtin, Name: r.Name, StudentTarget: access.IsStudentLike(r.Builtin, r.Grants)}
		switch err := guard.CheckSource(q.By, r); {
		case errors.Is(err, domain.ErrNotAssignable):
			choice.Verdict = model.RoleNeverGiven
		case errors.Is(err, domain.ErrTargetRoleNotAllowed):
			choice.Verdict = model.RoleBeyondCaller
		}
		out = append(out, choice)
	}
	return out, nil
}
