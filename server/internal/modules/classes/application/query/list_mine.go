package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
)

type ListMine struct {
	UserID string
}

type ListMineHandler struct {
	*support.Service
}

func (s ListMineHandler) Handle(ctx context.Context, q ListMine) ([]domain.MyClass, error) {
	return s.Repo.ListMine(ctx, q.UserID)
}
