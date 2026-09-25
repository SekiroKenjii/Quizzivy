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

type PreviewResult = domain.PreviewPaper

type PreviewHandler struct {
	*support.Service
}

func (s PreviewHandler) Handle(ctx context.Context, q Preview) (PreviewResult, error) {
	return s.Repo.Preview(ctx, q.TestID, q.Version)
}
