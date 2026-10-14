// Package application is the data export's use cases: asking for an export,
// reading its state and fetching its file once, and the worker that builds
// the file.
package application

import (
	"time"

	"quizzivy/internal/modules/exports/application/command"
	"quizzivy/internal/modules/exports/application/internal/support"
	"quizzivy/internal/modules/exports/application/ports"
	"quizzivy/internal/modules/exports/application/query"
	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module the API serves.
type Application struct {
	Commands Commands
	Queries  Queries
}

// Commands are the module's writes.
type Commands struct {
	Request  cqrs.CommandHandler[command.Request, domain.Export]
	Download cqrs.CommandHandler[command.Download, command.Downloaded]
}

// Queries are the module's reads.
type Queries struct {
	Get cqrs.QueryHandler[query.Get, domain.Export]
}

// New builds the module over repo, with the files kept in store and the
// worker woken through wake. zone dates the name of a downloaded file.
func New(repo domain.Repository, store ports.Store, wake ports.Wake, zone *time.Location) *Application {
	if zone == nil {
		zone = time.UTC
	}
	service := &support.Service{Repo: repo, Store: store, Wake: wake, Now: time.Now, Zone: zone}
	return &Application{
		Commands: Commands{
			Request:  command.RequestHandler{Service: service},
			Download: command.DownloadHandler{Service: service},
		},
		Queries: Queries{Get: query.GetHandler{Service: service}},
	}
}
