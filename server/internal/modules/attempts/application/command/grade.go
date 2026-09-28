package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// Grade marks a paper on an assignment Scope reaches, as GraderID.
type Grade struct {
	AttemptID string
	GraderID  string
	Items     []domain.GradeItem
	Scope     access.Scope
}

type GradeHandler struct {
	*support.Review
}

func (r GradeHandler) Handle(ctx context.Context, cmd Grade) (domain.Score, error) {
	return r.Repo.Grade(ctx, cmd.Scope, cmd.AttemptID, cmd.GraderID, cmd.Items)
}
