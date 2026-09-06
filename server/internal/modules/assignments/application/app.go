package application

import (
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

type Commands struct {
	Create cqrs.CommandHandler[command.Create, domain.Assignment]
	Reopen cqrs.CommandHandler[command.Reopen, domain.Assignment]
	Update cqrs.CommandHandler[command.Update, domain.Assignment]
}

type Queries struct {
	Facets        cqrs.QueryHandler[query.Facets, domain.Facets]
	ForStudent    cqrs.QueryHandler[query.ForStudent, domain.StudentSections]
	Get           cqrs.QueryHandler[query.Get, domain.Assignment]
	List          cqrs.QueryHandler[query.List, query.ListResult]
	StudentDetail cqrs.QueryHandler[query.StudentDetail, domain.StudentDetail]
}

func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{
			Create: command.CreateHandler{Service: service},
			Reopen: command.ReopenHandler{Service: service},
			Update: command.UpdateHandler{Service: service},
		},
		Queries: Queries{
			Facets:        query.FacetsHandler{Service: service},
			ForStudent:    query.ForStudentHandler{Service: service},
			Get:           query.GetHandler{Service: service},
			List:          query.ListHandler{Service: service},
			StudentDetail: query.StudentDetailHandler{Service: service},
		},
		service: service,
	}
}
