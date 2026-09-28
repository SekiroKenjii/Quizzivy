package query

import (
	"context"
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/access"
)

// Summary reads the teacher's home over what Scope reaches; a zero Scope
// reads nothing.
type Summary struct {
	Scope access.Scope
}

type SummaryHandler struct {
	*support.Service
}

func (s SummaryHandler) Handle(ctx context.Context, q Summary) (domain.Summary, error) {
	return s.Repo.Summary(ctx, q.Scope)
}
