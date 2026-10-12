package ports

import (
	"context"
	notificationsquery "quizzivy/internal/modules/notifications/application/query"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

// Zones resolves the caller's IANA calendar zone.
type Zones interface {
	ZoneOf(ctx context.Context, userID string) (string, error)
}

// UnreadNotifications is the notifications module's own unread summary query.
type UnreadNotifications = cqrs.QueryHandler[notificationsquery.Summary, notificationsdomain.Summary]
