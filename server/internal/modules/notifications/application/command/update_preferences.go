package command

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// UpdatePreferences replaces UserID's switches with Preferences, which must
// hold every event exactly once, and answers them as stored, in
// domain.Events' order.
type UpdatePreferences struct {
	UserID      string
	Preferences []domain.Preference
}

type UpdatePreferencesHandler struct {
	*support.Service
}

func (s UpdatePreferencesHandler) Handle(ctx context.Context, cmd UpdatePreferences) ([]domain.Preference, error) {
	if cmd.UserID == "" {
		return nil, domain.ErrNoRecipient
	}
	whole, err := domain.WholeSet(cmd.Preferences)
	if err != nil {
		return nil, err
	}
	stored, err := s.Repo.SavePreferences(ctx, cmd.UserID, whole)
	if err != nil {
		return nil, err
	}
	return domain.WithDefaults(stored), nil
}
