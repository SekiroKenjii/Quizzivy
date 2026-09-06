package wiring

import (
	dashboardapp "quizzivy/internal/modules/dashboard/application"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	"quizzivy/internal/platform/db"
)

func dashboard(dbx db.Context) dashboardhttp.Dashboard {
	return dashboardhttp.NewDashboard(dashboardapp.New(dashboardrepo.NewPostgres(dbx)))
}
