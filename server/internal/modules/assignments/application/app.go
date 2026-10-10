package application

import (
	"log/slog"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/application/ports"
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

// WithNotifier supplies the notifier that tells students their close was
// moved, and the logger a failed notification is written to. Without a
// notifier the commands move closes and tell nobody.
func (a *Application) WithNotifier(notifier ports.Notifier, logger *slog.Logger) *Application {
	a.service.Notifier = notifier
	a.service.Logger = logger
	return a
}

type Commands struct {
	Delete         cqrs.CommandHandler[command.Delete, cqrs.Nothing]
	Create         cqrs.CommandHandler[command.Create, domain.Assignment]
	Duplicate      cqrs.CommandHandler[command.Duplicate, domain.Assignment]
	Reopen         cqrs.CommandHandler[command.Reopen, domain.Assignment]
	Update         cqrs.CommandHandler[command.Update, domain.Assignment]
	Extend         cqrs.CommandHandler[command.Extend, domain.Assignment]
	SetOverrides   cqrs.CommandHandler[command.SetOverrides, []domain.StudentOverride]
	DeleteOverride cqrs.CommandHandler[command.DeleteOverride, cqrs.Nothing]
}

type Queries struct {
	Facets        cqrs.QueryHandler[query.Facets, domain.Facets]
	ForStudent    cqrs.QueryHandler[query.ForStudent, domain.StudentSections]
	Get           cqrs.QueryHandler[query.Get, domain.Assignment]
	List          cqrs.QueryHandler[query.List, query.ListResult]
	Overrides     cqrs.QueryHandler[query.Overrides, []domain.StudentOverride]
	StudentDetail cqrs.QueryHandler[query.StudentDetail, domain.StudentDetail]
}

func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{
			Delete:         command.DeleteHandler{Service: service},
			Create:         command.CreateHandler{Service: service},
			Duplicate:      command.DuplicateHandler{Service: service},
			Reopen:         command.ReopenHandler{Service: service},
			Update:         command.UpdateHandler{Service: service},
			Extend:         command.ExtendHandler{Service: service},
			SetOverrides:   command.SetOverridesHandler{Service: service},
			DeleteOverride: command.DeleteOverrideHandler{Service: service},
		},
		Queries: Queries{
			Facets:        query.FacetsHandler{Service: service},
			ForStudent:    query.ForStudentHandler{Service: service},
			Get:           query.GetHandler{Service: service},
			List:          query.ListHandler{Service: service},
			Overrides:     query.OverridesHandler{Service: service},
			StudentDetail: query.StudentDetailHandler{Service: service},
		},
		service: service,
	}
}
