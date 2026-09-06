package query

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
)

type Facets struct {
	Input domain.ListInput
}

type FacetsHandler struct {
	*support.Service
}

func (s FacetsHandler) Handle(ctx context.Context, q Facets) (domain.Facets, error) {
	return s.Repo.Facets(ctx, q.Input)
}
