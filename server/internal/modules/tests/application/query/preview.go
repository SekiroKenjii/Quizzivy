package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type Preview struct {
	TestID  string
	Version int
}

type PreviewResult struct {
	Total     int
	Questions []domain.PreviewQuestion
}

type PreviewHandler struct {
	*support.Service
}

func (s PreviewHandler) Handle(ctx context.Context, q Preview) (PreviewResult, error) {
	r0, r1, err := s.Repo.Preview(ctx, q.TestID, q.Version)
	return PreviewResult{Total: r0, Questions: r1}, err
}
