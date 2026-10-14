package command

import (
	"context"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/shared/cqrs"
)

// StampActivity notes that a user made an authenticated request (DG-54).
type StampActivity struct {
	UserID string
}

// StampActivityHandler hands the stamp to the collector, which throttles and batches it.
type StampActivityHandler struct {
	*support.Activity
}

func (h StampActivityHandler) Handle(_ context.Context, cmd StampActivity) (cqrs.Nothing, error) {
	h.Stamp(cmd.UserID)
	return cqrs.Nothing{}, nil
}
