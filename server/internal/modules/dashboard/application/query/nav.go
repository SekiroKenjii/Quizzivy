package query

import (
	"context"
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
	notificationsquery "quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/shared/access"
)

// Nav reads only the shell counts the caller may see.
type Nav struct {
	Scope    access.Scope
	CanGrade bool
}

type NavHandler struct{ *support.Service }

func (s NavHandler) Handle(ctx context.Context, q Nav) (domain.Nav, error) {
	if s.Notifications == nil {
		return domain.Nav{}, domain.ErrNotificationsUnavailable
	}
	live, err := s.Repo.LiveAssignments(ctx, q.Scope, s.Now())
	if err != nil {
		return domain.Nav{}, err
	}
	out := domain.Nav{LiveAssignments: &live}
	if q.CanGrade {
		count, err := s.Repo.AnswersToGrade(ctx, q.Scope)
		if err != nil {
			return domain.Nav{}, err
		}
		out.AnswersToGrade = &count
	}
	notifications, err := s.Notifications.Handle(ctx, notificationsquery.Summary{UserID: q.Scope.UserID})
	out.UnreadNotifications = notifications.UnreadNotifications
	return out, err
}
