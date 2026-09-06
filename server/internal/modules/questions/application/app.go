package application

import (
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/internal/support"
	"quizzivy/internal/modules/questions/application/ports"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

type Commands struct {
	AddTags   cqrs.CommandHandler[command.AddTags, int]
	Create    cqrs.CommandHandler[command.Create, domain.Question]
	Delete    cqrs.CommandHandler[command.Delete, cqrs.Nothing]
	Duplicate cqrs.CommandHandler[command.Duplicate, domain.Question]
	Update    cqrs.CommandHandler[command.Update, domain.Question]
}

type Queries struct {
	Counts              cqrs.QueryHandler[query.Counts, query.CountsResult]
	Facets              cqrs.QueryHandler[query.Facets, domain.TypeFacets]
	Get                 cqrs.QueryHandler[query.Get, domain.Question]
	GetIncludingDeleted cqrs.QueryHandler[query.GetIncludingDeleted, domain.Question]
	List                cqrs.QueryHandler[query.List, query.ListResult]
	Tags                cqrs.QueryHandler[query.Tags, []string]
}

func New(repo domain.Repository, media ports.MediaKinds) *Application {
	service := support.NewService(repo, media)
	return &Application{
		Commands: Commands{
			AddTags:   command.AddTagsHandler{Service: service},
			Create:    command.CreateHandler{Service: service},
			Delete:    command.DeleteHandler{Service: service},
			Duplicate: command.DuplicateHandler{Service: service},
			Update:    command.UpdateHandler{Service: service},
		},
		Queries: Queries{
			Counts:              query.CountsHandler{Service: service},
			Facets:              query.FacetsHandler{Service: service},
			Get:                 query.GetHandler{Service: service},
			GetIncludingDeleted: query.GetIncludingDeletedHandler{Service: service},
			List:                query.ListHandler{Service: service},
			Tags:                query.TagsHandler{Service: service},
		},
		service: service,
	}
}
