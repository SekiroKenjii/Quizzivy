package application

import (
	"time"

	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

// SetClock replaces the clock Prune counts the retention from.
func (a *Application) SetClock(now func() time.Time) {
	a.service.Now = now
}

type Commands struct {
	Notify            cqrs.CommandHandler[command.Notify, cqrs.Nothing]
	MaterialiseDue    cqrs.CommandHandler[command.MaterialiseDue, int]
	MarkRead          cqrs.CommandHandler[command.MarkRead, cqrs.Nothing]
	MarkAllRead       cqrs.CommandHandler[command.MarkAllRead, cqrs.Nothing]
	UpdatePreferences cqrs.CommandHandler[command.UpdatePreferences, []domain.Preference]
	Prune             cqrs.CommandHandler[command.Prune, int64]
}

type Queries struct {
	List        cqrs.QueryHandler[query.List, domain.Page]
	Summary     cqrs.QueryHandler[query.Summary, domain.Summary]
	Preferences cqrs.QueryHandler[query.Preferences, []domain.Preference]
}

func New(repo domain.Repository) *Application {
	service := support.NewService(repo)
	return &Application{
		Commands: Commands{
			Notify:            command.NotifyHandler{Service: service},
			MaterialiseDue:    command.MaterialiseDueHandler{Service: service},
			MarkRead:          command.MarkReadHandler{Service: service},
			MarkAllRead:       command.MarkAllReadHandler{Service: service},
			UpdatePreferences: command.UpdatePreferencesHandler{Service: service},
			Prune:             command.PruneHandler{Service: service},
		},
		Queries: Queries{
			List:        query.ListHandler{Service: service},
			Summary:     query.SummaryHandler{Service: service},
			Preferences: query.PreferencesHandler{Service: service},
		},
		service: service,
	}
}
