package wiring

import (
	"quizzivy/internal/core/adapters"
	dashboardapp "quizzivy/internal/modules/dashboard/application"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/platform/db"
)

func dashboard(dbx db.Context, notifications *notificationsapp.Application) dashboardhttp.Dashboard {
	return dashboardhttp.NewDashboard(dashboardapp.New(dashboardrepo.NewPostgres(dbx)).WithZones(adapters.DefaultZone{}).WithNotifications(notifications.Queries.Summary))
}
