package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

type Facets struct {
	Query string
}

type FacetsHandler struct {
	*support.Service
}

func (s FacetsHandler) Handle(ctx context.Context, q Facets) (domain.Facets, error) {
	return s.Repo.Facets(ctx, q.Query)
}
