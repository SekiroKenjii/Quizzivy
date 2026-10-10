package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/content"
)

type Flag struct {
	Request   domain.Request
	AttemptID string
	Flagged   bool
	Reason    string
}

type FlagHandler struct {
	*support.Service
}

func (s FlagHandler) Handle(ctx context.Context, cmd Flag) (domain.Attempt, error) {
	return s.Store.Flag(ctx, cmd.Request, cmd.AttemptID, cmd.Flagged, content.NFC(cmd.Reason), s.Now())
}
