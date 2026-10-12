package wiring

import (
	"quizzivy/internal/core/adapters"
	dashboardapp "quizzivy/internal/modules/dashboard/application"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/platform/db"
)

func dashboard(dbx db.Context, notifications *notificationsapp.Application, identity *identityapp.Application) dashboardhttp.Dashboard {
	return dashboardhttp.NewDashboard(dashboardapp.New(dashboardrepo.NewPostgres(dbx)).WithZones(adapters.ProfileZone{Query: identity.Queries.EffectiveZone}).WithNotifications(notifications.Queries.Summary))
}
