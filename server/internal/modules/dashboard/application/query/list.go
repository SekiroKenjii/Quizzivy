package query

import (
	"context"
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/paging"
)

type List struct {
	Query domain.ListQuery
}

type ListResult struct {
	Items []domain.Recent
	Page  paging.Page
}

type ListHandler struct {
	*support.Service
}

func (s ListHandler) Handle(ctx context.Context, q List) (ListResult, error) {
	r0, r1, err := s.Repo.List(ctx, q.Query)
	return ListResult{Items: r0, Page: r1}, err
}
