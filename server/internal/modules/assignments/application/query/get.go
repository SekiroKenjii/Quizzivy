package query

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/access"
)

// Get reads one assignment Scope reaches; another teacher's answers
// ErrNotFound.
type Get struct {
	ID    string
	Scope access.Scope
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Assignment, error) {
	return s.Repo.Get(ctx, q.Scope, q.ID)
}
