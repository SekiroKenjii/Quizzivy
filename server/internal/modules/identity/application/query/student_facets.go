package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

type StudentFacets struct {
	Query domain.StudentQuery
}

type StudentFacetsHandler struct {
	*support.Students
}

func (s StudentFacetsHandler) Handle(ctx context.Context, q StudentFacets) (domain.StudentFacets, error) {
	return s.Repo.Facets(ctx, q.Query)
}
