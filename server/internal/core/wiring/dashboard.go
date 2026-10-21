package wiring

import (
	"quizzivy/internal/core/adapters"
	aiapp "quizzivy/internal/modules/ai/application"
	auditapp "quizzivy/internal/modules/audit/application"
	dashboardapp "quizzivy/internal/modules/dashboard/application"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	organizationapp "quizzivy/internal/modules/organization/application"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

type dashboardModules struct {
	notifications *notificationsapp.Application
	identity      *identityapp.Application
	organization  *organizationapp.Application
	audit         *auditapp.Application
	ai            *aiapp.Application
}

// dashboard reads the Admin console's figures in the centre's own zone, against its usage limits. The AI credits are offered to it only while the assistant is configured, so the Overview shows no credit-request row and the usage no AI figure while it is off.
func dashboard(cfg config.Config, dbx db.Context, m dashboardModules, aiOn bool) (dashboardhttp.Dashboard, error) {
	zone, err := config.AppLocation()
	if err != nil {
		return dashboardhttp.Dashboard{}, err
	}
	app := dashboardapp.New(dashboardrepo.NewPostgres(dbx)).
		WithZones(adapters.ProfileZone{Query: m.identity.Queries.EffectiveZone}).
		WithNotifications(m.notifications.Queries.Summary).
		WithAdmin(m.organization.Queries.Branding, m.audit.Queries.Recent, cfg.Limits, zone)
	if aiOn {
		app.WithCredits(m.ai.Queries.AdminCredits)
	}
	return dashboardhttp.NewDashboard(app), nil
}
