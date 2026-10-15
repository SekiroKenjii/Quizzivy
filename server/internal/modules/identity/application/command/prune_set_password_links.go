package command

import (
	"context"
	"errors"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// PruneSetPasswordLinks deletes at most Limit set-password links that ended
// more than domain.SetPasswordLinksKept ago, and returns how many went. It is
// intended for a scheduled call, repeated while it returns Limit.
type PruneSetPasswordLinks struct {
	Limit int
}

// PruneSetPasswordLinksHandler prunes through the links store.
type PruneSetPasswordLinksHandler struct {
	*support.Service
}

func (h PruneSetPasswordLinksHandler) Handle(ctx context.Context, c PruneSetPasswordLinks) (int, error) {
	if c.Limit <= 0 {
		return 0, errors.New("prune set-password links: limit must be positive")
	}
	return h.Links.Prune(ctx, h.Now().Add(-domain.SetPasswordLinksKept), c.Limit)
}
