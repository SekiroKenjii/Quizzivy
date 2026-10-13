package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/cqrs"
)

// Release returns a reservation's credits to the pool and the teacher, for a call that was never billed. Releasing a reservation that is no longer held, because it was released already or the sweep returned it, does nothing.
type Release struct {
	Reservation domain.Reservation
}

type ReleaseHandler struct {
	*support.Service
}

func (h ReleaseHandler) Handle(ctx context.Context, cmd Release) (cqrs.Nothing, error) {
	_, err := h.Repo.Release(ctx, cmd.Reservation)
	return cqrs.Nothing{}, err
}
