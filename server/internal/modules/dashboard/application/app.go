package application

import (
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/application/ports"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/cqrs"
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
}

func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{},
		Queries: Queries{
			Nav:     query.NavHandler{Service: service},
			List:    query.ListHandler{Service: service},
			Summary: query.SummaryHandler{Service: service},
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
