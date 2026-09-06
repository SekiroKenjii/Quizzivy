package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
)

type TotalBytes struct {
	Kind *domain.Kind
}

type TotalBytesHandler struct {
	*support.Service
}

func (s TotalBytesHandler) Handle(ctx context.Context, q TotalBytes) (int64, error) {
	return s.Repo.TotalBytes(ctx, q.Kind)
}
