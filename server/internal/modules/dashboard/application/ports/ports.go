package ports

import (
	"context"
	aiquery "quizzivy/internal/modules/ai/application/query"
	aidomain "quizzivy/internal/modules/ai/domain"
	auditquery "quizzivy/internal/modules/audit/application/query"
	auditdomain "quizzivy/internal/modules/audit/domain"
	notificationsquery "quizzivy/internal/modules/notifications/application/query"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/shared/cqrs"
)

// Zones resolves the caller's IANA calendar zone.
type Zones interface {
	ZoneOf(ctx context.Context, userID string) (string, error)
}

// UnreadNotifications is the notifications module's own unread summary query.
type UnreadNotifications = cqrs.QueryHandler[notificationsquery.Summary, notificationsdomain.Summary]

// Branding is the organization module's own query for the centre's name.
type Branding = cqrs.QueryHandler[organizationquery.Branding, organizationmodel.Branding]

// RecentChanges is the audit module's own query for its newest entries.
type RecentChanges = cqrs.QueryHandler[auditquery.Recent, []auditdomain.Entry]

// AICredits is the AI module's own query for the Admin's view of the month, the one GET /admin/ai/credits answers.
type AICredits = cqrs.QueryHandler[aiquery.AdminCredits, aidomain.AdminCredits]
