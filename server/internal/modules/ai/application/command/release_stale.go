package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
)

// ReleaseStale returns every reservation held longer than domain.StaleAfter, which a call that died with its process leaves behind. It is idempotent, so two machines or two runs release what the first did not.
type ReleaseStale struct{}

type ReleaseStaleHandler struct {
	*support.Service
}

func (h ReleaseStaleHandler) Handle(ctx context.Context, _ ReleaseStale) (domain.Released, error) {
	return h.Repo.ReleaseStale(ctx, h.Now().Add(-domain.StaleAfter))
}
