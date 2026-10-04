package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
)

func ids(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = assignment
	}
	return out
}

func TestListAsksTheStoreForTheCallersPage(t *testing.T) {
	store := only(t, "List")
	store.page = domain.Page{Items: []domain.Notification{{ID: assignment, Kind: domain.ResultReady}}, NextBefore: assignment}
	page, err := over(store).Queries.List.Handle(context.Background(), query.List{UserID: reader, Before: assignment, Limit: 7})
	if err != nil {
		t.Fatal(err)
	}
	if store.query != (domain.ListQuery{UserID: reader, Before: assignment, Limit: 7}) {
		t.Errorf("the store was asked %+v", store.query)
	}
	if len(page.Items) != 1 || page.NextBefore != assignment {
		t.Errorf("the page is %+v, want the store's", page)
	}
}

func TestNothingIsReadOrMarkedWithoutACaller(t *testing.T) {
	store := only(t, "")
	app := over(store)
	ctx := context.Background()
	_, list := app.Queries.List.Handle(ctx, query.List{})
	_, summary := app.Queries.Summary.Handle(ctx, query.Summary{})
	_, preferences := app.Queries.Preferences.Handle(ctx, query.Preferences{})
	_, mark := app.Commands.MarkRead.Handle(ctx, command.MarkRead{IDs: ids(1)})
	_, all := app.Commands.MarkAllRead.Handle(ctx, command.MarkAllRead{})
	_, update := app.Commands.UpdatePreferences.Handle(ctx, command.UpdatePreferences{Preferences: domain.WithDefaults(nil)})
	for label, err := range map[string]error{
		"List": list, "Summary": summary, "Preferences": preferences, "MarkRead": mark, "MarkAllRead": all, "UpdatePreferences": update,
	} {
		if !errors.Is(err, domain.ErrNoRecipient) {
			t.Errorf("%s without a user: err = %v, want ErrNoRecipient", label, err)
		}
	}
	if store.calls != 0 {
		t.Errorf("the store was called %d times without a user", store.calls)
	}
}

func TestSummaryCountsTheCallersUnread(t *testing.T) {
	store := only(t, "Unread")
	store.unread = 3
	summary, err := over(store).Queries.Summary.Handle(context.Background(), query.Summary{UserID: reader})
	if err != nil || summary.UnreadNotifications != 3 || store.userID != reader {
		t.Errorf("summary %+v for %q (%v), want 3 unread for the caller", summary, store.userID, err)
	}
}

func TestMarkReadTakesOneToAHundredIds(t *testing.T) {
	for n, want := range map[int]error{0: domain.ErrNoIDs, 1: nil, 100: nil, 101: domain.ErrTooManyIDs} {
		allowed := ""
		if want == nil {
			allowed = "MarkRead"
		}
		store := only(t, allowed)
		_, err := over(store).Commands.MarkRead.Handle(context.Background(), command.MarkRead{UserID: reader, IDs: ids(n)})
		if !errors.Is(err, want) {
			t.Errorf("%d ids: err = %v, want %v", n, err, want)
		}
		if want == nil && (store.userID != reader || len(store.ids) != n) {
			t.Errorf("%d ids: the store marked %d for %q", n, len(store.ids), store.userID)
		}
	}
}

func TestMarkAllReadMarksOnlyThroughTheCallersOwnStatement(t *testing.T) {
	store := only(t, "MarkAllRead")
	if _, err := over(store).Commands.MarkAllRead.Handle(context.Background(), command.MarkAllRead{UserID: reader}); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || store.userID != reader || store.ids != nil {
		t.Errorf("the store saw %d calls for %q with ids %v", store.calls, store.userID, store.ids)
	}
}

func TestPreferencesFillTheFiveDefaults(t *testing.T) {
	store := only(t, "Preferences")
	got, err := over(store).Queries.Preferences.Handle(context.Background(), query.Preferences{UserID: reader})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || store.userID != reader {
		t.Fatalf("%d preferences for %q, want the five switches for the caller", len(got), store.userID)
	}
	for i, event := range domain.Events() {
		if got[i] != (domain.Preference{Event: event, InApp: true, Email: false}) {
			t.Errorf("%s defaults to %+v, want in the app and not by email", event, got[i])
		}
	}

	store = only(t, "Preferences")
	store.stored = []domain.Preference{{Event: domain.EventAssignmentClosing, InApp: false, Email: true}}
	got, err = over(store).Queries.Preferences.Handle(context.Background(), query.Preferences{UserID: reader})
	if err != nil {
		t.Fatal(err)
	}
	if got[2] != store.stored[0] || !got[0].InApp || got[0].Email {
		t.Errorf("a stored switch did not win, or a default moved: %+v", got)
	}
}

func TestUpdatePreferencesStoresTheWholeSetInTheSwitchesOrder(t *testing.T) {
	given := domain.WithDefaults(nil)
	given[1].InApp, given[4].Email = false, true
	want := slices.Clone(given)
	slices.Reverse(given)

	store := only(t, "SavePreferences")
	got, err := over(store).Commands.UpdatePreferences.Handle(context.Background(), command.UpdatePreferences{UserID: reader, Preferences: given})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(store.saved, want) || store.userID != reader {
		t.Errorf("the store was handed %+v for %q, want %+v", store.saved, store.userID, want)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the answer is %+v, want the stored set %+v", got, want)
	}
}

func TestUpdatePreferencesRefusesARepeatedAnUnknownAndAMissingEvent(t *testing.T) {
	repeated := domain.WithDefaults(nil)
	repeated[3].Event = domain.EventAttemptSubmitted
	unknown := domain.WithDefaults(nil)
	unknown[0].Event = "summary.weekly"
	for label, c := range map[string]struct {
		given []domain.Preference
		want  error
	}{
		"a repeated event": {repeated, domain.ErrEventsNotOnceEach},
		"an unknown event": {unknown, domain.ErrUnknownEvent},
		"a missing event":  {domain.WithDefaults(nil)[:4], domain.ErrEventsNotOnceEach},
	} {
		store := only(t, "")
		_, err := over(store).Commands.UpdatePreferences.Handle(context.Background(), command.UpdatePreferences{UserID: reader, Preferences: c.given})
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", label, err, c.want)
		}
		if store.calls != 0 {
			t.Errorf("%s: the store was written", label)
		}
	}
}

func TestPruneDeletesWhatWasWrittenMoreThan180DaysAgo(t *testing.T) {
	now := time.Date(2026, 10, 4, 9, 30, 0, 0, time.UTC)
	store := only(t, "DeleteBefore")
	store.deleted = 12
	app := over(store)
	app.SetClock(func() time.Time { return now })
	deleted, err := app.Commands.Prune.Handle(context.Background(), command.Prune{})
	if err != nil || deleted != 12 {
		t.Fatalf("deleted %d (%v), want the store's 12", deleted, err)
	}
	if want := now.Add(-180 * 24 * time.Hour); !store.cutoff.Equal(want) {
		t.Errorf("the cutoff is %s, want %s: exactly 180 days before now", store.cutoff, want)
	}
}
