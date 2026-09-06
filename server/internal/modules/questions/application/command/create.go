package command

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type Create struct {
	Request domain.WriteRequest
}

type CreateHandler struct {
	*support.Service
}

func (s CreateHandler) Handle(ctx context.Context, cmd Create) (domain.Question, error) {
	return s.Write(ctx, cmd.Request, false)
}
