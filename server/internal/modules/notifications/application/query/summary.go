package query

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// Summary counts what UserID's shell shows: their unread notifications.
type Summary struct {
	UserID string
}

type SummaryHandler struct {
	*support.Service
}

func (s SummaryHandler) Handle(ctx context.Context, q Summary) (domain.Summary, error) {
	if q.UserID == "" {
		return domain.Summary{}, domain.ErrNoRecipient
	}
	unread, err := s.Repo.Unread(ctx, q.UserID)
	return domain.Summary{UnreadNotifications: unread}, err
}
