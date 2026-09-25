package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
)

type Review struct {
	AttemptID string
}

type ReviewHandler struct {
	*support.Review
}

func (r ReviewHandler) Handle(ctx context.Context, q Review) (domain.Review, error) {
	review, err := r.Repo.Get(ctx, q.AttemptID)
	if err != nil {
		return domain.Review{}, err
	}
	return r.PaperContext(ctx, review)
}
