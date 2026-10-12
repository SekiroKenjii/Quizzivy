package repositories

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/identity/domain"
	"time"
)

// EffectiveZone reads only the eligible account's zone and defaults only a stored NULL.
func (s *Users) EffectiveZone(ctx context.Context, userID string) (string, error) {
	var zone *string
	var disabled *time.Time
	err := s.QueryRow(ctx, `SELECT time_zone, disabled_at FROM app.users WHERE id = $1::uuid`, userID).Scan(&zone, &disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrUserNotFound
	}
	if err != nil {
		return "", err
	}
	if disabled != nil {
		return "", domain.ErrAccountDisabled
	}
	if zone == nil {
		return "Asia/Ho_Chi_Minh", nil
	}
	if *zone == "" || *zone == "Local" {
		return "", domain.ErrTimeZoneInvalid
	}
	if _, err := time.LoadLocation(*zone); err != nil {
		return "", domain.ErrTimeZoneInvalid
	}
	return *zone, nil
}
