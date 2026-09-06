package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
)

type Update struct {
	Request domain.Request
	Input   domain.WriteInput
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Assignment, error) {
	return s.Repo.Update(ctx, cmd.Request, cmd.Input)
}
