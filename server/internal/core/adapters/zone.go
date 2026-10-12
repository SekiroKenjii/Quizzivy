package adapters

import (
	"context"
	identityquery "quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/shared/cqrs"
)

// ProfileZone supplies the actor's calendar zone through the identity query port.
type ProfileZone struct {
	Query cqrs.QueryHandler[identityquery.EffectiveZone, string]
}

func (a ProfileZone) ZoneOf(ctx context.Context, userID string) (string, error) {
	return a.Query.Handle(ctx, identityquery.EffectiveZone{UserID: userID})
}
