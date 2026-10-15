package command

import (
	"context"
	"errors"
	"time"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// PruneActivityDays deletes at most Limit activity days older than
// domain.ActivityDaysKept local days, and returns how many went. It is
// intended for a scheduled call, repeated while it returns Limit.
type PruneActivityDays struct {
	Limit int
}

// PruneActivityDaysHandler prunes through the activity store.
type PruneActivityDaysHandler struct {
	*support.Activity
}

func (h PruneActivityDaysHandler) Handle(ctx context.Context, c PruneActivityDays) (int, error) {
	if c.Limit <= 0 {
		return 0, errors.New("prune activity days: limit must be positive")
	}
	local := h.Now().In(h.Zone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return h.Store.PruneDays(ctx, today.AddDate(0, 0, -domain.ActivityDaysKept), c.Limit)
}
