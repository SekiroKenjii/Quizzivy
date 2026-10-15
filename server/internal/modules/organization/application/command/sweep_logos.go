package command

import (
	"context"
	"errors"
	"fmt"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/domain"
)

// SweepLogos removes the stored logos that neither the light nor the dark
// logo of the settings names and that were written more than domain.LogoGrace
// ago, looking at no more than Limit stored objects. Without logo storage it
// removes nothing. Every removable logo of the batch is tried, and the first
// failure is the error. Intended for a scheduled call, repeated while it
// returns Limit.
type SweepLogos struct {
	Limit int
}

// SweptLogos is how many stored logos a SweepLogos removed, and how many
// objects it looked at.
type SweptLogos struct {
	Removed int
	Seen    int
}

type SweepLogosHandler struct {
	*support.Service
}

func (h SweepLogosHandler) Handle(ctx context.Context, c SweepLogos) (SweptLogos, error) {
	if c.Limit <= 0 {
		return SweptLogos{}, errors.New("sweep logos: limit must be positive")
	}
	if h.Logos == nil {
		return SweptLogos{}, nil
	}
	stored, err := h.Logos.List(ctx, support.LogoPrefix, c.Limit)
	if err != nil {
		return SweptLogos{}, fmt.Errorf("sweep logos: list the stored logos: %w", err)
	}
	out := SweptLogos{Seen: len(stored)}
	settings, err := h.Repo.Read(ctx)
	if err != nil {
		return out, fmt.Errorf("sweep logos: read the settings: %w", err)
	}
	named := map[string]bool{}
	for _, logo := range []*domain.Logo{settings.Profile.LightLogo, settings.Profile.DarkLogo} {
		if logo != nil {
			named[support.LogoKey(*logo)] = true
		}
	}
	cutoff := h.Now().Add(-domain.LogoGrace)
	var first error
	for _, object := range stored {
		if named[object.Key] || !object.WrittenAt.Before(cutoff) {
			continue
		}
		if err := h.Logos.Delete(ctx, object.Key); err != nil {
			first = errors.Join(first, fmt.Errorf("sweep logos: delete a stored logo: %w", err))
			continue
		}
		out.Removed++
	}
	return out, first
}
