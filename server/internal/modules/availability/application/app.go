// Package application is the availability use cases: what the API tells anyone who asks whether it is, or will soon be, down for maintenance.
package application

import (
	"log/slog"
	"time"

	"quizzivy/internal/modules/availability/application/internal/support"
	"quizzivy/internal/modules/availability/application/query"
	"quizzivy/internal/modules/availability/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module.
type Application struct {
	Queries Queries
	service *support.Service
}

// Queries are the module's reads.
type Queries struct {
	CurrentWindow cqrs.QueryHandler[query.CurrentWindow, domain.Status]
}

// New builds the application over the repository, with one snapshot shared by
// every caller.
func New(repo domain.Repository, logger *slog.Logger) *Application {
	service := &support.Service{Snapshot: support.NewSnapshot(repo, logger)}
	return &Application{
		Queries: Queries{CurrentWindow: query.CurrentWindowHandler{Service: service}},
		service: service,
	}
}

// SetClock replaces the time source. Tests only.
func (a *Application) SetClock(now func() time.Time) { a.service.Snapshot.SetClock(now) }
