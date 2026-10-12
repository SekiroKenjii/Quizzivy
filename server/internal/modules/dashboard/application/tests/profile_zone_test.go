package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	identityquery "quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"
)

func TestProfileZoneAdapterReachesCalendarWithoutChangingClockOrScope(t *testing.T) {
	repo := &store{t: t}
	app := application.New(repo)
	now := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	app.SetClock(func() time.Time { return now })
	scope := access.Scope{UserID: "actor", All: false}
	for _, zone := range []string{"America/New_York", "UTC", "Asia/Tokyo"} {
		app.WithZones(adapters.ProfileZone{Query: cqrs.HandlerFunc[identityquery.EffectiveZone, string](func(_ context.Context, q identityquery.EffectiveZone) (string, error) {
			if q.UserID != "actor" {
				t.Fatal(q.UserID)
			}
			return zone, nil
		})})
		if _, err := app.Queries.Summary.Handle(context.Background(), query.Summary{Scope: scope, Range: "7d"}); err != nil {
			t.Fatal(err)
		}
		if repo.home.Zone != zone || repo.home.Now != now || repo.home.Scope != scope || repo.home.Days != 7 {
			t.Fatalf("calendar=%+v", repo.home)
		}
	}
	boom := errors.New("identity query failed")
	app.WithZones(adapters.ProfileZone{Query: cqrs.HandlerFunc[identityquery.EffectiveZone, string](func(context.Context, identityquery.EffectiveZone) (string, error) { return "", boom })})
	if _, err := app.Queries.Summary.Handle(context.Background(), query.Summary{Scope: scope}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
