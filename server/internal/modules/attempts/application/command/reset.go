package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/content"
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
	return s.Store.Reset(ctx, cmd.Request, cmd.AttemptID, content.NFC(cmd.Reason), s.Now())
}
