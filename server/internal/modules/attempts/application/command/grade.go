package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Grade struct {
	AttemptID string
	GraderID  string
	Items     []domain.GradeItem
}

type GradeHandler struct {
	*support.Review
}

func (r GradeHandler) Handle(ctx context.Context, cmd Grade) (domain.Score, error) {
	return r.Repo.Grade(ctx, cmd.AttemptID, cmd.GraderID, cmd.Items)
}
