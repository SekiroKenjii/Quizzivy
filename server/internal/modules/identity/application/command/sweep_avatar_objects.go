package command

import (
	"context"
	"errors"
	"fmt"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
)

const (
	avatarSweepPage   = 100
	avatarSweepRounds = 10
)

// SweepAvatarObjects removes the profile photo objects of at most Limit
// anonymised accounts not swept yet, in id order, and returns how many
// accounts it marked (D-050). For each account it deletes every object under
// domain.AvatarPrefix + id + "/", listing again while a list comes back full,
// and only then marks the account. An account whose list or delete fails is
// not marked, so the next run tries it again; the rest of the batch is still
// tried and the first failure is the error. A failure to mark stops the batch.
// It is intended for a scheduled call, repeated while it returns Limit.
type SweepAvatarObjects struct {
	Limit int
}

// SweepAvatarObjectsHandler sweeps through the store of swept accounts and
// the photos' object store.
type SweepAvatarObjectsHandler struct {
	*support.Service
	Sweeps  domain.AvatarSweeps
	Objects ports.AvatarObjects
}

func (h SweepAvatarObjectsHandler) Handle(ctx context.Context, c SweepAvatarObjects) (int, error) {
	if c.Limit <= 0 {
		return 0, errors.New("sweep avatar objects: limit must be positive")
	}
	if h.Objects == nil {
		return 0, nil
	}
	due, err := h.Sweeps.Due(ctx, c.Limit)
	if err != nil {
		return 0, fmt.Errorf("sweep avatar objects: %w", err)
	}
	marked := 0
	var first error
	for _, id := range due {
		if err := ctx.Err(); err != nil {
			return marked, errors.Join(first, err)
		}
		if err := h.empty(ctx, domain.AvatarPrefix+id+"/"); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if err := h.Sweeps.Mark(ctx, id, h.Now()); err != nil {
			return marked, errors.Join(first, fmt.Errorf("sweep avatar objects: %w", err))
		}
		marked++
	}
	return marked, first
}

func (h SweepAvatarObjectsHandler) empty(ctx context.Context, prefix string) error {
	for range avatarSweepRounds {
		keys, err := h.Objects.List(ctx, prefix, avatarSweepPage)
		if err != nil {
			return fmt.Errorf("sweep avatar objects: list an account's photos: %w", err)
		}
		for _, key := range keys {
			if err := h.Objects.Delete(ctx, key); err != nil {
				return fmt.Errorf("sweep avatar objects: delete an account's photo: %w", err)
			}
		}
		if len(keys) < avatarSweepPage {
			return nil
		}
	}
	return errors.New("sweep avatar objects: an account's photos did not run out")
}
