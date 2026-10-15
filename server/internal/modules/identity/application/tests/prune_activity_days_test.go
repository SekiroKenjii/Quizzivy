package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
)

func TestPruningActivityKeepsFourHundredLocalDays(t *testing.T) {
	log := &activityLog{}
	app, _ := activityApp(t, log, time.Date(2026, time.October, 14, 18, 30, 0, 0, time.UTC))

	removed, err := app.Commands.PruneActivityDays.Handle(context.Background(), command.PruneActivityDays{Limit: 500})
	if err != nil || removed != 500 {
		t.Fatalf("removed %d (%v), want what the store removed", removed, err)
	}
	if len(log.pruned) != 1 || log.pruned[0].limit != 500 {
		t.Fatalf("store was asked %+v, want one call with limit 500", log.pruned)
	}
	want := time.Date(2025, time.September, 10, 0, 0, 0, 0, time.UTC)
	if got := log.pruned[0].before; !got.Equal(want) {
		t.Errorf("days before %v go, want %v: 400 days before 15 October 2026 in Ho Chi Minh City, where 18:30 UTC is already the 15th", got, want)
	}
	if want.AddDate(0, 0, domain.ActivityDaysKept).Format("2006-01-02") != "2026-10-15" {
		t.Errorf("the expected cutoff is not %d days before the local day", domain.ActivityDaysKept)
	}
}

func TestPruningActivityNeedsAPositiveLimit(t *testing.T) {
	log := &activityLog{}
	app, _ := activityApp(t, log, time.Now())
	for _, limit := range []int{0, -1} {
		if _, err := app.Commands.PruneActivityDays.Handle(context.Background(), command.PruneActivityDays{Limit: limit}); err == nil {
			t.Errorf("limit %d was accepted", limit)
		}
	}
	if len(log.pruned) != 0 {
		t.Errorf("the store was asked %+v for a limit it cannot honour", log.pruned)
	}
}

func TestAFailedPruneIsTheStoresError(t *testing.T) {
	boom := errors.New("the database is away")
	app, _ := activityApp(t, &activityLog{err: boom}, time.Now())
	if _, err := app.Commands.PruneActivityDays.Handle(context.Background(), command.PruneActivityDays{Limit: 10}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the store's", err)
	}
}

func TestWithoutActivityThereIsNothingToPrune(t *testing.T) {
	if app := application.New(nil, nil, 0, nil, nil); app.Commands.PruneActivityDays != nil {
		t.Error("an application that records no activity offers to prune it")
	}
}
