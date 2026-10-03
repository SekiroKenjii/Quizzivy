package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
)

type Get struct {
	ID    string
	Scope access.Scope
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Test, error) {
	return s.Repo.Get(ctx, q.Scope, q.ID)
}
