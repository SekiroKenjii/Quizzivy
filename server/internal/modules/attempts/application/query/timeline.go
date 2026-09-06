package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Timeline struct {
	AttemptID string
}

type TimelineHandler struct {
	*support.Integrity
}

func (i TimelineHandler) Handle(ctx context.Context, q Timeline) (domain.Timeline, error) {
	return i.Repo.Timeline(ctx, q.AttemptID)
}
