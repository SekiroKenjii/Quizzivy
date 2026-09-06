package query

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

type GetIncludingDeleted struct {
	ID string
}

type GetIncludingDeletedHandler struct {
	*support.Service
}

func (s GetIncludingDeletedHandler) Handle(ctx context.Context, q GetIncludingDeleted) (domain.Question, error) {
	return s.Repo.GetIncludingDeleted(ctx, q.ID)
}
