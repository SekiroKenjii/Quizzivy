package application

import (
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands  Commands
	Queries   Queries
	publisher *support.Publisher
	service   *support.Service
}

type Commands struct {
	Create    cqrs.CommandHandler[command.Create, domain.Test]
	Duplicate cqrs.CommandHandler[command.Duplicate, domain.Test]
	Publish   cqrs.CommandHandler[command.Publish, domain.PublishedVersion]
	Update    cqrs.CommandHandler[command.Update, domain.Test]
}

type Queries struct {
	Facets       cqrs.QueryHandler[query.Facets, domain.StatusFacets]
	Get          cqrs.QueryHandler[query.Get, domain.Test]
	List         cqrs.QueryHandler[query.List, query.ListResult]
	ListVersions cqrs.QueryHandler[query.ListVersions, []domain.Version]
	Preview      cqrs.QueryHandler[query.Preview, query.PreviewResult]
	Tags         cqrs.QueryHandler[query.Tags, []string]
}

func New(repo domain.Repository) *Application {
	publisher := support.NewPublisher(repo)
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{
			Create:    command.CreateHandler{Service: service},
			Duplicate: command.DuplicateHandler{Service: service},
			Publish:   command.PublishHandler{Publisher: publisher},
			Update:    command.UpdateHandler{Service: service},
		},
		Queries: Queries{
			Facets:       query.FacetsHandler{Service: service},
			Get:          query.GetHandler{Service: service},
			List:         query.ListHandler{Service: service},
			ListVersions: query.ListVersionsHandler{Service: service},
			Preview:      query.PreviewHandler{Service: service},
			Tags:         query.TagsHandler{Service: service},
		},
		publisher: publisher,
		service:   service,
	}
}
