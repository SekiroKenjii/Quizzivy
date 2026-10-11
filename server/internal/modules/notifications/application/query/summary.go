package query

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
)

// Summary counts what UserID's shell shows: their unread notifications, of
// Audience's kinds when it names one, and of every kind when it is empty. An
// Audience that is neither the teacher's nor the student's is refused.
type Summary struct {
	UserID   string
	Audience domain.Audience
}

type SummaryHandler struct {
	*support.Service
}

func (s SummaryHandler) Handle(ctx context.Context, q Summary) (domain.Summary, error) {
	switch {
	case q.UserID == "":
		return domain.Summary{}, domain.ErrNoRecipient
	case !q.Audience.Valid():
		return domain.Summary{}, domain.ErrUnknownAudience
	}
	unread, err := s.Repo.Unread(ctx, q.UserID, q.Audience)
	return domain.Summary{UnreadNotifications: unread}, err
}
