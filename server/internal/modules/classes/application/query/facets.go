package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

// Facets counts the classes Scope reaches that match Query.
type Facets struct {
	Query string
	Scope access.Scope
}

type FacetsHandler struct {
	*support.Service
}

func (s FacetsHandler) Handle(ctx context.Context, q Facets) (domain.Facets, error) {
	return s.Repo.Facets(ctx, q.Scope, q.Query)
}
