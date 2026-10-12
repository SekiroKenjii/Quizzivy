package adapters_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	identityquery "quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/shared/cqrs"
	"testing"
)

func TestProfileZoneForwardsOnlyActorAndPropagatesQueryErrors(t *testing.T) {
	for _, zone := range []string{"Asia/Ho_Chi_Minh", "Asia/Tokyo", "UTC", "America/New_York"} {
		adapter := adapters.ProfileZone{Query: cqrs.HandlerFunc[identityquery.EffectiveZone, string](func(_ context.Context, q identityquery.EffectiveZone) (string, error) {
			if q.UserID != "actor" {
				t.Fatalf("actor=%q", q.UserID)
			}
			return zone, nil
		})}
		if got, err := adapter.ZoneOf(context.Background(), "actor"); err != nil || got != zone {
			t.Fatalf("zone=%q %v", got, err)
		}
	}
	boom := errors.New("zone database unavailable")
	adapter := adapters.ProfileZone{Query: cqrs.HandlerFunc[identityquery.EffectiveZone, string](func(context.Context, identityquery.EffectiveZone) (string, error) { return "", boom })}
	if got, err := adapter.ZoneOf(context.Background(), "actor"); got != "" || !errors.Is(err, boom) {
		t.Fatalf("outage defaulted=%q %v", got, err)
	}
}
