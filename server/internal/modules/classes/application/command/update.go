package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

// Update edits a class's own fields.
type Update struct {
	ClassID string
	Input   domain.UpdateInput
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Class, error) {
	return s.Repo.Update(ctx, cmd.ClassID, cmd.Input)
}
