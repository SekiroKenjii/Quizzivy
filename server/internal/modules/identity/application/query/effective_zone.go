package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
)

// EffectiveZone identifies the caller whose stored calendar zone is requested.
type EffectiveZone struct{ UserID string }

// EffectiveZoneHandler exposes only the account's effective calendar zone.
type EffectiveZoneHandler struct{ *support.Service }

func (s EffectiveZoneHandler) Handle(ctx context.Context, q EffectiveZone) (string, error) {
	return s.Users.EffectiveZone(ctx, q.UserID)
}
