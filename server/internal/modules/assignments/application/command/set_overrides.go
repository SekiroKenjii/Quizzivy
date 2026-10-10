package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
)

// SetOverrides gives the students in Input an override, or changes the one
// they have, on the assignment Request names.
type SetOverrides struct {
	Request domain.Request
	Input   domain.OverrideInput
}

type SetOverridesHandler struct {
	*support.Service
}

func (s SetOverridesHandler) Handle(ctx context.Context, cmd SetOverrides) ([]domain.StudentOverride, error) {
	return s.Repo.SetOverrides(ctx, cmd.Request, cmd.Input)
}
