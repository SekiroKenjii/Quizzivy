package query

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// Preferences reads every switch for UserID in domain.Events' order: the
// saved value where there is one, the default where there is not.
type Preferences struct {
	UserID string
}

type PreferencesHandler struct {
	*support.Service
}

func (s PreferencesHandler) Handle(ctx context.Context, q Preferences) ([]domain.Preference, error) {
	if q.UserID == "" {
		return nil, domain.ErrNoRecipient
	}
	stored, err := s.Repo.Preferences(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	return domain.WithDefaults(stored), nil
}
