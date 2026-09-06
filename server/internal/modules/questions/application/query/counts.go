package query

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type Counts struct {
	Input domain.ListInput
}

type CountsResult struct {
	Total    int
	Filtered int
}

type CountsHandler struct {
	*support.Service
}

func (s CountsHandler) Handle(ctx context.Context, q Counts) (CountsResult, error) {
	r0, r1, err := s.Repo.Counts(ctx, q.Input)
	return CountsResult{Total: r0, Filtered: r1}, err
}
