package command

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
)

type Create struct {
	Request domain.Request
	Input   domain.WriteInput
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Assignment, error) {
	return s.Repo.Create(ctx, cmd.Request, cmd.Input)
}
