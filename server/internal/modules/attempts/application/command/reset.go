package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Reset struct {
	Request   domain.Request
	AttemptID string
	Reason    string
}

type ResetHandler struct {
	*support.Service
}

func (s ResetHandler) Handle(ctx context.Context, cmd Reset) (domain.Attempt, error) {
	return s.Store.Reset(ctx, cmd.Request, cmd.AttemptID, cmd.Reason, s.Now())
}
