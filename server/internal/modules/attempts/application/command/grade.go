package command

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/content"
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
	items := cmd.Items
	if items != nil {
		items = make([]domain.GradeItem, len(cmd.Items))
		for i, item := range cmd.Items {
			item.Comment = content.NFCPtr(item.Comment)
			items[i] = item
		}
	}
	return r.Repo.Grade(ctx, cmd.Scope, cmd.AttemptID, cmd.GraderID, items)
}
