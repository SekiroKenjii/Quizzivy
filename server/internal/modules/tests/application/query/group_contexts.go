package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
)

// GroupContexts reads learner-safe shared context for a frozen version; callers authorize access to that version.
type GroupContexts struct {
	VersionID string
}

type GroupContextsHandler struct {
	*support.Service
}

func (s GroupContextsHandler) Handle(ctx context.Context, q GroupContexts) ([]domain.PreviewGroup, error) {
	return s.Repo.GroupContexts(ctx, q.VersionID)
}
