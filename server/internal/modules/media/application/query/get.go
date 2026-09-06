package query

import (
	"context"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
)

// Get resolves one live asset, so another package can render an attachment
// without reaching into media's store.
type Get struct {
	ID string
}

type GetHandler struct {
	*support.Service
}

func (s GetHandler) Handle(ctx context.Context, q Get) (domain.Asset, error) {
	return s.Repo.Get(ctx, q.ID)
}
