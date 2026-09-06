package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type Facets struct {
	Input domain.ListInput
}

type FacetsHandler struct {
	*support.Service
}

func (s FacetsHandler) Handle(ctx context.Context, q Facets) (domain.StatusFacets, error) {
	return s.Repo.Facets(ctx, q.Input)
}
