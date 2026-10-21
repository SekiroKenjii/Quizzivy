package application

import (
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/application/ports"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/limits"
	"time"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

type Commands struct {
}

type Queries struct {
	Nav     cqrs.QueryHandler[query.Nav, domain.Nav]
	List    cqrs.QueryHandler[query.List, query.ListResult]
	Summary cqrs.QueryHandler[query.Summary, domain.Summary]

	AdminOverview cqrs.QueryHandler[query.AdminOverview, domain.AdminOverview]
	AdminSummary  cqrs.QueryHandler[query.AdminSummary, domain.AdminSummary]
	Usage         cqrs.QueryHandler[query.Usage, domain.Usage]
}

func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{},
		Queries: Queries{
			Nav:     query.NavHandler{Service: service},
			List:    query.ListHandler{Service: service},
			Summary: query.SummaryHandler{Service: service},

			AdminOverview: query.AdminOverviewHandler{Service: service},
			AdminSummary:  query.AdminSummaryHandler{Service: service},
			Usage:         query.UsageHandler{Service: service},
		},
		service: service,
	}
}

// SetClock replaces the application clock used by the new dashboard readings.
func (a *Application) SetClock(now func() time.Time) { a.service.Now = now }

// WithZones sets the actor-zone port for calendar readings.
func (a *Application) WithZones(zones ports.Zones) *Application { a.service.Zones = zones; return a }

// WithNotifications sets the notifications summary port for the nav reading.
func (a *Application) WithNotifications(port ports.UnreadNotifications) *Application {
	a.service.Notifications = port
	return a
}

// WithAdmin sets what the Admin readings are drawn from: the organization's name, the audit log's newest entries, the deploy's overrides of the usage limits and the zone whose days they count. A nil location counts days in the centre's default zone, and a nil recent leaves the Overview's recent changes empty.
func (a *Application) WithAdmin(branding ports.Branding, recent ports.RecentChanges, env limits.Env, loc *time.Location) *Application {
	a.service.Branding, a.service.Recent, a.service.LimitEnv = branding, recent, env
	if loc != nil {
		a.service.Location = loc
	}
	return a
}

// WithCredits sets the AI credits query behind the Overview's credit-request row and the usage's AI figures. Without it both are absent.
func (a *Application) WithCredits(credits ports.AICredits) *Application {
	a.service.Credits = credits
	return a
}
