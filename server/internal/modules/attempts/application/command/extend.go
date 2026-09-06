package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Extend struct {
	Request   domain.Request
	AttemptID string
	Minutes   int
	Reason    string
}

type ExtendHandler struct {
	*support.Service
}

func (s ExtendHandler) Handle(ctx context.Context, cmd Extend) (domain.Attempt, error) {
	return s.Store.Extend(ctx, cmd.Request, cmd.AttemptID, cmd.Minutes, cmd.Reason, s.Now())
}
