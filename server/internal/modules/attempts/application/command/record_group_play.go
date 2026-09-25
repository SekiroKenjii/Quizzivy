package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type RecordGroupPlay struct {
	Input domain.GroupPlayInput
}

type RecordGroupPlayHandler struct {
	*support.Service
}

func (s RecordGroupPlayHandler) Handle(ctx context.Context, cmd RecordGroupPlay) (domain.GroupPlays, error) {
	return s.Store.RecordGroupPlay(ctx, cmd.Input, s.Now())
}
