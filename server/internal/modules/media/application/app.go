package application

import (
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/application/model"
	"quizzivy/internal/modules/media/application/ports"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/cqrs"
	"time"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

func (a *Application) SignedURLTTL() time.Duration {
	return a.service.SignedURLTTL()
}

func (a *Application) WithSignedURLTTL(ttl time.Duration) *Application {
	a.service.WithSignedURLTTL(ttl)
	return a
}

type Commands struct {
	Delete cqrs.CommandHandler[command.Delete, cqrs.Nothing]
	Upload cqrs.CommandHandler[command.Upload, domain.Asset]
}

type Queries struct {
	Get            cqrs.QueryHandler[query.Get, domain.Asset]
	List           cqrs.QueryHandler[query.List, query.ListResult]
	MintForStudent cqrs.QueryHandler[query.MintForStudent, model.SignedURLResult]
	SignedURL      cqrs.QueryHandler[query.SignedURL, string]
	TotalBytes     cqrs.QueryHandler[query.TotalBytes, int64]
}

func New(repo domain.Repository, object ports.ObjectStore, probe ports.AudioProbe) *Application {
	service := support.NewService(repo, object, probe)
	return &Application{
		Commands: Commands{
			Delete: command.DeleteHandler{Service: service},
			Upload: command.UploadHandler{Service: service},
		},
		Queries: Queries{
			Get:            query.GetHandler{Service: service},
			List:           query.ListHandler{Service: service},
			MintForStudent: query.MintForStudentHandler{Service: service},
			SignedURL:      query.SignedURLHandler{Service: service},
			TotalBytes:     query.TotalBytesHandler{Service: service},
		},
		service: service,
	}
}
