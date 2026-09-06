package command

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/cqrs"
)

type Delete struct {
	Input domain.DeleteInput
}

type DeleteHandler struct {
	*support.Service
}

func (s DeleteHandler) Handle(ctx context.Context, cmd Delete) (cqrs.Nothing, error) {
	if cmd.Input.Now.IsZero() {
		cmd.Input.Now = s.Now()
	}
	return cqrs.Nothing{}, s.Repo.SoftDelete(ctx, cmd.Input)
}
