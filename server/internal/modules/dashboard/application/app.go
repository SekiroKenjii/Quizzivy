package application

import (
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/cqrs"
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
	List    cqrs.QueryHandler[query.List, query.ListResult]
	Summary cqrs.QueryHandler[query.Summary, domain.Summary]
}

func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{},
		Queries: Queries{
			List:    query.ListHandler{Service: service},
			Summary: query.SummaryHandler{Service: service},
		},
		service: service,
	}
}
