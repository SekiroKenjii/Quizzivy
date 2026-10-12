//go:build integration

package repositories_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
)

func preferences(t *testing.T, app *application.Application, userID string) []domain.Preference {
	t.Helper()
	got, err := app.Queries.Preferences.Handle(context.Background(), query.Preferences{UserID: userID})
	if err != nil {
		t.Fatalf("preferences: %v", err)
	}
	return got
}

func save(t *testing.T, app *application.Application, userID string, prefs []domain.Preference) []domain.Preference {
	t.Helper()
	got, err := app.Commands.UpdatePreferences.Handle(context.Background(), command.UpdatePreferences{UserID: userID, Preferences: prefs})
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
	return got
}

func savedRows(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	n, err := db.Count(context.Background(), pool, `SELECT count(*) FROM app.notification_preferences WHERE user_id = $1::uuid`, userID)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func savedAt(t *testing.T, pool *pgxpool.Pool, userID string) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(context.Background(), `
		SELECT updated_at FROM app.notification_preferences WHERE user_id = $1::uuid AND event = 'attempt.submitted'`, userID).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestASwitchDefaultsUntilItIsStoredAndTheStoredOneWins(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID, other := newUser(t, pool, "teacher"), newUser(t, pool, "teacher")
	defaults := domain.WithDefaults(nil)

	if got := preferences(t, app, userID); !slices.Equal(got, defaults) {
		t.Errorf("with no row the switches are %+v, want the five defaults", got)
	}
	if n := savedRows(t, pool, userID); n != 0 {
		t.Errorf("reading the switches stored %d rows, want none", n)
	}

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.notification_preferences (user_id, event, in_app, email)
		VALUES ($1::uuid, 'assignment.closing', false, true)`, userID); err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(defaults)
	want[2] = domain.Preference{Event: domain.EventAssignmentClosing, InApp: false, Email: true}
	if got := preferences(t, app, userID); !slices.Equal(got, want) {
		t.Errorf("with one stored row the switches are %+v, want %+v", got, want)
	}

	given := []domain.Preference{
		{Event: domain.EventResultReady, InApp: false, Email: false},
		{Event: domain.EventAssignmentDueSoon, InApp: true, Email: true},
		{Event: domain.EventAssignmentClosing, InApp: true, Email: false},
		{Event: domain.EventAttemptFlagged, InApp: false, Email: true},
		{Event: domain.EventAttemptSubmitted, InApp: true, Email: false},
	}
	stored := slices.Clone(given)
	slices.Reverse(stored)
	if got := save(t, app, userID, given); !slices.Equal(got, stored) {
		t.Errorf("saving answered %+v, want the stored set in the switches' order %+v", got, stored)
	}
	if got := preferences(t, app, userID); !slices.Equal(got, stored) {
		t.Errorf("after saving the switches are %+v, want %+v", got, stored)
	}
	if n := savedRows(t, pool, userID); n != 5 {
		t.Errorf("%d rows after saving, want five", n)
	}

	firstSaved := savedAt(t, pool, userID)
	time.Sleep(20 * time.Millisecond)
	stored[0].InApp, stored[4].Email = false, true
	if got := save(t, app, userID, stored); !slices.Equal(got, stored) {
		t.Errorf("saving again answered %+v, want %+v", got, stored)
	}
	if got := preferences(t, app, userID); !slices.Equal(got, stored) {
		t.Errorf("after saving again the switches are %+v, want %+v", got, stored)
	}
	if n := savedRows(t, pool, userID); n != 5 {
		t.Errorf("%d rows after saving twice, want five", n)
	}
	if again := savedAt(t, pool, userID); !again.After(firstSaved) {
		t.Errorf("saving again left updated_at at %v, was %v", again, firstSaved)
	}

	if got := preferences(t, app, other); !slices.Equal(got, defaults) {
		t.Errorf("another user's switches are %+v after the caller saved theirs, want the defaults", got)
	}
	if n := savedRows(t, pool, other); n != 0 {
		t.Errorf("saving stored %d rows for another user", n)
	}
}
