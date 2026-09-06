package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Finish struct {
	AttemptID string
}

type FinishHandler struct {
	*support.Review
}

func (r FinishHandler) Handle(ctx context.Context, cmd Finish) (domain.Attempt, error) {
	return r.Repo.Finish(ctx, cmd.AttemptID)
}
