package application

import (
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/application/ports"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/cqrs"
	"time"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands  Commands
	Queries   Queries
	publisher *support.Publisher
	service   *support.Service
	groups    *support.Groups
}

type Commands struct {
	CreateGroup            cqrs.CommandHandler[command.CreateGroup, domain.StoredGroup]
	UpdateGroup            cqrs.CommandHandler[command.UpdateGroup, domain.StoredGroup]
	CopyGroup              cqrs.CommandHandler[command.CopyGroup, domain.StoredGroup]
	ArchiveGroup           cqrs.CommandHandler[command.ArchiveGroup, domain.StoredGroup]
	DeleteGroup            cqrs.CommandHandler[command.DeleteGroup, cqrs.Nothing]
	CreateDraftFromVersion cqrs.CommandHandler[command.CreateDraftFromVersion, domain.Test]
	SetCurrentVersion      cqrs.CommandHandler[command.SetCurrentVersion, domain.Test]
	DeleteVersion          cqrs.CommandHandler[command.DeleteVersion, cqrs.Nothing]
	Delete                 cqrs.CommandHandler[command.Delete, cqrs.Nothing]
	Create                 cqrs.CommandHandler[command.Create, domain.Test]
	Duplicate              cqrs.CommandHandler[command.Duplicate, domain.Test]
	Publish                cqrs.CommandHandler[command.Publish, domain.PublishedVersion]
	Update                 cqrs.CommandHandler[command.Update, domain.Test]
}

type Queries struct {
	Group         cqrs.QueryHandler[query.Group, domain.StoredGroup]
	Groups        cqrs.QueryHandler[query.Groups, query.GroupsResult]
	GroupContexts cqrs.QueryHandler[query.GroupContexts, []domain.PreviewGroup]
	Facets        cqrs.QueryHandler[query.Facets, domain.StatusFacets]
	Get           cqrs.QueryHandler[query.Get, domain.Test]
	List          cqrs.QueryHandler[query.List, query.ListResult]
	ListVersions  cqrs.QueryHandler[query.ListVersions, []domain.Version]
	Preview       cqrs.QueryHandler[query.Preview, query.PreviewResult]
	Tags          cqrs.QueryHandler[query.Tags, []string]
}

func New(repo domain.Repository) *Application {
	publisher := support.NewPublisher(repo)
	service := support.NewService(repo)
	groups := &support.Groups{Now: time.Now}
	return &Application{
		Commands: Commands{
			CreateGroup:            command.CreateGroupHandler{Groups: groups},
			UpdateGroup:            command.UpdateGroupHandler{Groups: groups},
			CopyGroup:              command.CopyGroupHandler{Groups: groups},
			ArchiveGroup:           command.ArchiveGroupHandler{Groups: groups},
			DeleteGroup:            command.DeleteGroupHandler{Groups: groups},
			CreateDraftFromVersion: command.CreateDraftFromVersionHandler{Service: service},
			SetCurrentVersion:      command.SetCurrentVersionHandler{Service: service},
			DeleteVersion:          command.DeleteVersionHandler{Service: service},
			Delete:                 command.DeleteHandler{Service: service},
			Create:                 command.CreateHandler{Service: service},
			Duplicate:              command.DuplicateHandler{Service: service},
			Publish:                command.PublishHandler{Publisher: publisher},
			Update:                 command.UpdateHandler{Service: service},
		},
		Queries: Queries{
			Group:         query.GroupHandler{Groups: groups},
			Groups:        query.GroupsHandler{Groups: groups},
			GroupContexts: query.GroupContextsHandler{Service: service},
			Facets:        query.FacetsHandler{Service: service},
			Get:           query.GetHandler{Service: service},
			List:          query.ListHandler{Service: service},
			ListVersions:  query.ListVersionsHandler{Service: service},
			Preview:       query.PreviewHandler{Service: service},
			Tags:          query.TagsHandler{Service: service},
		},
		publisher: publisher,
		service:   service,
		groups:    groups,
	}
}

// WithGroups configures complete-context authoring; absent dependencies leave these operations unavailable.
func (a *Application) WithGroups(repo domain.GroupRepository, media ports.GroupMediaKinds) *Application {
	a.groups.Repo = repo
	a.groups.Media = media
	return a
}
