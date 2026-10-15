package command

import (
	"context"
	"errors"
	"fmt"

	"quizzivy/internal/modules/exports/application/internal/support"
)

// Expire removes the files of at most Limit exports whose time has passed and
// marks them expired, oldest first. A file is deleted before its row is
// marked, so a failure between the two leaves a row the next call deals with
// again; the other exports of the batch are still seen to, and the first
// failure is the error. Intended for a scheduled call, repeated while it
// returns Limit.
type Expire struct {
	Limit int
}

// Expired is how many exports an Expire marked.
type Expired struct {
	Marked int
}

type ExpireHandler struct {
	*support.Service
}

func (h ExpireHandler) Handle(ctx context.Context, c Expire) (Expired, error) {
	if c.Limit <= 0 {
		return Expired{}, errors.New("expire exports: limit must be positive")
	}
	lapsed, err := h.Repo.Lapsed(ctx, c.Limit)
	if err != nil {
		return Expired{}, err
	}
	var out Expired
	var first error
	for _, lapse := range lapsed {
		if err := h.Store.Delete(ctx, lapse.StorageKey); err != nil {
			first = errors.Join(first, fmt.Errorf("expire exports: delete the file of an export: %w", err))
			continue
		}
		marked, err := h.Repo.Expire(ctx, lapse.ID)
		if err != nil {
			first = errors.Join(first, err)
			continue
		}
		if marked {
			out.Marked++
		}
	}
	return out, first
}
