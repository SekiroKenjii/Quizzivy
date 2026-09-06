package query

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type Facets struct {
	Input domain.ListInput
}

type FacetsHandler struct {
	*support.Service
}

func (s FacetsHandler) Handle(ctx context.Context, q Facets) (domain.TypeFacets, error) {
	return s.Repo.Facets(ctx, q.Input)
}
