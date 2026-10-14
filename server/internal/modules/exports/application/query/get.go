package query

import (
	"context"

	"quizzivy/internal/modules/exports/application/internal/support"
	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/shared/actor"
)

// Get reads the export ID that Actor requested. Anyone else's export is
// domain.ErrNotFound, as a missing one is.
type Get struct {
	Actor actor.Actor
	ID    string
}

type GetHandler struct {
	*support.Service
}

func (h GetHandler) Handle(ctx context.Context, q Get) (domain.Export, error) {
	return h.Repo.Get(ctx, q.ID, q.Actor.ID)
}
