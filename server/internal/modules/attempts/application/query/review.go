package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// Review reads one paper on an assignment Scope reaches.
type Review struct {
	AttemptID string
	Scope     access.Scope
}

type ReviewHandler struct {
	*support.Review
}

func (r ReviewHandler) Handle(ctx context.Context, q Review) (domain.Review, error) {
	review, err := r.Repo.Get(ctx, q.Scope, q.AttemptID)
	if err != nil {
		return domain.Review{}, err
	}
	return r.PaperContext(ctx, review)
}
