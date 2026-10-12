package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
)

// Facets counts the library's tabs for the Scope and the Query of Input,
// whatever its Kind and Unused say.
type Facets struct {
	Input domain.ListInput
}

type FacetsHandler struct {
	*support.Service
}

func (s FacetsHandler) Handle(ctx context.Context, q Facets) (domain.Facets, error) {
	return s.Repo.Facets(ctx, q.Input)
}
