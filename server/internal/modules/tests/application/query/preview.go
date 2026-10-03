package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
)

type Preview struct {
	TestID  string
	Version int
	Scope   access.Scope
}

type PreviewResult = domain.PreviewPaper

type PreviewHandler struct {
	*support.Service
}

func (s PreviewHandler) Handle(ctx context.Context, q Preview) (PreviewResult, error) {
	return s.Repo.Preview(ctx, q.Scope, q.TestID, q.Version)
}
