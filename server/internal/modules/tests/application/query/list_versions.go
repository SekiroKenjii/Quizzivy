package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

type ListVersions struct {
	TestID string
}

type ListVersionsHandler struct {
	*support.Service
}

func (s ListVersionsHandler) Handle(ctx context.Context, q ListVersions) ([]domain.Version, error) {
	return s.Repo.ListVersions(ctx, q.TestID)
}
