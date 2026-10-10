package query

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/access"
)

// Overrides reads the overrides on one assignment Scope reaches, for the
// students Scope reaches; another teacher's assignment answers ErrNotFound.
type Overrides struct {
	AssignmentID string
	Scope        access.Scope
}

type OverridesHandler struct {
	*support.Service
}

func (s OverridesHandler) Handle(ctx context.Context, q Overrides) ([]domain.StudentOverride, error) {
	return s.Repo.Overrides(ctx, q.Scope, q.AssignmentID)
}
