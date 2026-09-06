package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/cqrs"
)

type Flush struct {
	Input domain.FlushInput
}

type FlushHandler struct {
	*support.Service
}

func (s FlushHandler) Handle(ctx context.Context, cmd Flush) (cqrs.Nothing, error) {
	return cqrs.Nothing{}, s.Store.Flush(ctx, cmd.Input, s.Now())
}
