package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/paging"
)

type List struct {
	Input domain.ListInput
}

type ListResult struct {
	Items []domain.Test
	Page  paging.Page
}

type ListHandler struct {
	*support.Service
}

func (s ListHandler) Handle(ctx context.Context, q List) (ListResult, error) {
	r0, r1, err := s.Repo.List(ctx, q.Input)
	return ListResult{Items: r0, Page: r1}, err
}
