package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"testing"
	"time"
)

func TestPreferencePatchCarriesFalseAndNestedReplacementWithoutDefaults(t *testing.T) {
	for _, p := range []domain.Preferences{{}, {CompactTables: profileBool(false), LargerTestText: profileBool(false)}, {AssignmentDefaults: &domain.AssignmentDefaults{ShowScore: profileBool(false)}}} {
		repo := &profileUsers{}
		app := application.New(repo, nil, time.Hour, nil, nil)
		_, err := app.Commands.UpdatePreferences.Handle(context.Background(), command.UpdatePreferences{UserID: "a", Preferences: p})
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(repo.prefs.Patch) != string(want) || repo.prefs.UserID != "a" {
			t.Fatalf("patch=%s want%s", repo.prefs.Patch, want)
		}
	}
	for _, p := range []domain.Preferences{{Theme: profileName("neon")}, {AssignmentDefaults: &domain.AssignmentDefaults{DurationMinutes: profileInt(0)}}, {AssignmentDefaults: &domain.AssignmentDefaults{DurationMinutes: profileInt(601)}}} {
		repo := &profileUsers{}
		app := application.New(repo, nil, time.Hour, nil, nil)
		_, err := app.Commands.UpdatePreferences.Handle(context.Background(), command.UpdatePreferences{UserID: "a", Preferences: p})
		if !errors.Is(err, domain.ErrPreferencesInvalid) || repo.calls != 0 {
			t.Fatalf("error=%v calls=%d", err, repo.calls)
		}
	}
	boom := errors.New("write failed")
	repo := &profileUsers{err: boom}
	app := application.New(repo, nil, time.Hour, nil, nil)
	if _, err := app.Commands.UpdatePreferences.Handle(context.Background(), command.UpdatePreferences{UserID: "a"}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func profileInt(n int) *int { return &n }
