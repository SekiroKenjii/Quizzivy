package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// Timeline reads the integrity log of an attempt on an assignment Scope
// reaches.
type Timeline struct {
	AttemptID string
	Scope     access.Scope
}

type TimelineHandler struct {
	*support.Integrity
}

func (i TimelineHandler) Handle(ctx context.Context, q Timeline) (domain.Timeline, error) {
	return i.Repo.Timeline(ctx, q.Scope, q.AttemptID)
}
