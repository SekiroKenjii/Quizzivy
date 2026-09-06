package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

type Get struct {
	ClassID string
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Class, error) {
	return s.Repo.Get(ctx, q.ClassID)
}
