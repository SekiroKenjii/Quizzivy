package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
)

// Get reads one class Scope reaches; another teacher's answers ErrNotFound.
type Get struct {
	ClassID string
	Scope   access.Scope
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Class, error) {
	return s.Repo.Get(ctx, q.Scope, q.ClassID)
}
