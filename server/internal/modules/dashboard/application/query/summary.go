package query

import (
	"context"
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
)

type Summary struct {
}

type SummaryHandler struct {
	*support.Service
}

func (s SummaryHandler) Handle(ctx context.Context, _ Summary) (domain.Summary, error) {
	return s.Repo.Summary(ctx)
}
