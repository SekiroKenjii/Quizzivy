package command

import (
	"context"
	"encoding/json"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
)

// UpdatePreferences merges the caller's supplied top-level preference keys.
type UpdatePreferences struct {
	UserID      string
	Preferences domain.Preferences
	IP          string
	UserAgent   string
}

// UpdatePreferencesHandler keeps effective defaults out of the persisted patch.
type UpdatePreferencesHandler struct{ *support.Service }

func (s UpdatePreferencesHandler) Handle(ctx context.Context, cmd UpdatePreferences) (domain.Preferences, error) {
	if !validPreferences(cmd.Preferences) {
		return domain.Preferences{}, domain.ErrPreferencesInvalid
	}
	patch, err := json.Marshal(cmd.Preferences)
	if err != nil {
		return domain.Preferences{}, err
	}
	return s.Users.UpdatePreferences(ctx, domain.PreferencesRecord{UserID: cmd.UserID, Patch: patch, Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent)})
}

func validPreferences(p domain.Preferences) bool {
	if p.Theme != nil && *p.Theme != "light" && *p.Theme != "dark" && *p.Theme != "system" {
		return false
	}
	if p.AssignmentDefaults != nil && p.AssignmentDefaults.DurationMinutes != nil {
		duration := *p.AssignmentDefaults.DurationMinutes
		return duration >= 1 && duration <= 600
	}
	return true
}
