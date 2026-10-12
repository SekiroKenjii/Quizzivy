package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
)

func scanPreference(rows pgx.Rows) (domain.Preference, error) {
	var (
		p     domain.Preference
		event string
	)
	if err := rows.Scan(&event, &p.InApp, &p.Email); err != nil {
		return domain.Preference{}, err
	}
	p.Event = domain.Event(event)
	return p, nil
}

// Preferences returns the switches the user has saved, and only those: a
// switch never saved has no row.
func (p *Postgres) Preferences(ctx context.Context, userID string) ([]domain.Preference, error) {
	stored, err := db.QueryMany(ctx, p, `
		SELECT event, in_app, email FROM app.notification_preferences WHERE user_id = $1::uuid`,
		[]any{userID}, scanPreference)
	if err != nil {
		return nil, fmt.Errorf("notifications: preferences: %w", err)
	}
	return stored, nil
}

// SavePreferences stores prefs for the user in one statement, in the order
// given, and returns the rows as stored.
func (p *Postgres) SavePreferences(ctx context.Context, userID string, prefs []domain.Preference) ([]domain.Preference, error) {
	events := make([]string, len(prefs))
	inApp := make([]bool, len(prefs))
	email := make([]bool, len(prefs))
	for i, pref := range prefs {
		events[i], inApp[i], email[i] = string(pref.Event), pref.InApp, pref.Email
	}
	stored, err := db.QueryMany(ctx, p, `
		INSERT INTO app.notification_preferences (user_id, event, in_app, email)
		SELECT $1::uuid, given.event, given.in_app, given.email
		  FROM unnest($2::text[], $3::boolean[], $4::boolean[]) AS given(event, in_app, email)
		ON CONFLICT (user_id, event) DO UPDATE
		   SET in_app = EXCLUDED.in_app, email = EXCLUDED.email
		RETURNING event, in_app, email`,
		[]any{userID, events, inApp, email}, scanPreference)
	if err != nil {
		return nil, fmt.Errorf("notifications: save preferences: %w", err)
	}
	return stored, nil
}
