package query

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"
)

// Get reads one live bank question that Scope reaches; another owner's answers
// ErrNotFound, exactly as a missing one does.
type Get struct {
	ID    string
	Scope access.Scope
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Question, error) {
	return s.Repo.Get(ctx, q.Scope, q.ID)
}
