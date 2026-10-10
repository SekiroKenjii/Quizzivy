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
	reached, err := s.Store.Flush(ctx, cmd.Input, s.Now())
	if err != nil {
		return cqrs.Nothing{}, err
	}
	s.Announcer.Announce(ctx, reached)
	return cqrs.Nothing{}, nil
}
