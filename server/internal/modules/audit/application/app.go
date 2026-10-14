package application

import (
	"time"

	"quizzivy/internal/modules/audit/application/command"
	"quizzivy/internal/modules/audit/application/internal/support"
	"quizzivy/internal/modules/audit/application/ports"
	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

type Commands struct {
	StartExport cqrs.CommandHandler[command.StartExport, domain.ExportPlan]
}

type Queries struct {
	List        cqrs.QueryHandler[query.List, query.ListResult]
	Recent      cqrs.QueryHandler[query.Recent, []domain.Entry]
	ExportBatch cqrs.QueryHandler[query.ExportBatch, query.ExportBatchResult]
}

// New builds the module over repo. Without WithZones every viewer reads the
// centre's default zone, and without WithAvatars no actor has a photo.
func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{StartExport: command.StartExportHandler{Service: service}},
		Queries: Queries{
			List:        query.ListHandler{Service: service},
			Recent:      query.RecentHandler{Service: service},
			ExportBatch: query.ExportBatchHandler{Service: service},
		},
		service: service,
	}
}

// SetClock replaces the clock the ranges are counted from.
func (a *Application) SetClock(now func() time.Time) { a.service.Now = now }

// WithZones supplies the viewer's calendar zone, which the days of a range are
// counted in.
func (a *Application) WithZones(zones ports.Zones) *Application {
	a.service.Zones = zones
	return a
}

// WithAvatars supplies the signer of an actor's profile photo.
func (a *Application) WithAvatars(avatars ports.Avatars) *Application {
	a.service.Avatars = avatars
	return a
}
