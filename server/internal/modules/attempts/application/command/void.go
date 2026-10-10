package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/content"
)

type Void struct {
	Request   domain.Request
	AttemptID string
	Reason    string
}

type VoidHandler struct {
	*support.Service
}

func (s VoidHandler) Handle(ctx context.Context, cmd Void) (domain.Attempt, error) {
	return s.Store.Void(ctx, cmd.Request, cmd.AttemptID, content.NFC(cmd.Reason), s.Now())
}
