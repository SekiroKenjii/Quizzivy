package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/actor"
)

// Reserve holds Ceiling credits for one call of Feature before it reaches the provider, against the pool and the actor's allowance for the current month. It is refused with domain.ErrPaused for a paused teacher, before any lock is taken, and with *domain.Exhausted, naming the budget, when the allowance or the pool cannot cover the ceiling; nothing is written then. The caller owes the reservation to Settle, Release or Finish.
type Reserve struct {
	Actor   actor.Actor
	Feature domain.Feature
	Ceiling int64
}

type ReserveHandler struct {
	*support.Service
}

func (h ReserveHandler) Handle(ctx context.Context, cmd Reserve) (domain.Reservation, error) {
	if cmd.Actor.ID == "" {
		return domain.Reservation{}, domain.ErrNotATeacher
	}
	if !cmd.Feature.Known() {
		return domain.Reservation{}, domain.ErrUnknownFeature
	}
	if cmd.Ceiling < 1 || cmd.Ceiling > domain.MaxCeiling {
		return domain.Reservation{}, domain.ErrInvalidCeiling
	}
	return h.Repo.Reserve(ctx, domain.ReserveInput{
		Month: h.Month(), UserID: cmd.Actor.ID, Feature: cmd.Feature, Credits: cmd.Ceiling,
	})
}
