package query

import (
	"context"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/domain"
)

// GetIncludingDeleted resolves a question whether or not it is deleted, for the
// version snapshot path.
type GetIncludingDeleted struct {
	ID string
}

type GetIncludingDeletedHandler struct {
	*support.Service
}

func (s GetIncludingDeletedHandler) Handle(ctx context.Context, q GetIncludingDeleted) (domain.Question, error) {
	return s.Repo.GetIncludingDeleted(ctx, q.ID)
}
