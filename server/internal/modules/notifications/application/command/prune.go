package command

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// Prune deletes every notification first written more than domain.Retention
// ago and answers how many went. Intended for a scheduled call; a second run
// finds nothing.
type Prune struct{}

type PruneHandler struct {
	*support.Service
}

func (s PruneHandler) Handle(ctx context.Context, _ Prune) (int64, error) {
	return s.Repo.DeleteBefore(ctx, s.Now().Add(-domain.Retention))
}
