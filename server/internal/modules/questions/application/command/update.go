package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type Update struct {
	Request domain.WriteRequest
}

type UpdateHandler struct {
	*support.Service
}

func (s UpdateHandler) Handle(ctx context.Context, cmd Update) (domain.Question, error) {
	if cmd.Request.ID == "" {
		return domain.Question{}, domain.ErrNotFound
	}
	input, err := cmd.Request.Input.Composed()
	if err != nil {
		return domain.Question{}, err
	}
	cmd.Request.Input = input
	if err := cmd.Request.Input.ValidateAuthoring(); err != nil {
		return domain.Question{}, err
	}
	return s.Write(ctx, cmd.Request, true)
}
