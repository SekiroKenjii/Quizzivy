package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

// Update edits the own fields of a class Scope reaches.
type Update struct {
	ClassID string
	Input   domain.UpdateInput
	Scope   access.Scope
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Class, error) {
	return s.Repo.Update(ctx, cmd.Scope, cmd.ClassID, cmd.Input)
}
