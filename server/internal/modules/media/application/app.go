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

// WithImageProbe sets what reads an image's pixel size at upload; without
// one an image is stored with no size.
func (a *Application) WithImageProbe(images ports.ImageProbe) *Application {
	a.service.WithImageProbe(images)
	return a
}

// WithOwnerQuota sets the bytes one owner's library may hold. A non-positive
// value keeps the domain's default.
func (a *Application) WithOwnerQuota(bytes int64) *Application {
	a.service.WithOwnerQuota(bytes)
	return a
}

type Commands struct {
	Delete cqrs.CommandHandler[command.Delete, cqrs.Nothing]
	Update cqrs.CommandHandler[command.Update, domain.Asset]
	Upload cqrs.CommandHandler[command.Upload, domain.Asset]
}

type Queries struct {
	Facets         cqrs.QueryHandler[query.Facets, domain.Facets]
	Get            cqrs.QueryHandler[query.Get, domain.Asset]
	List           cqrs.QueryHandler[query.List, query.ListResult]
	MintForStudent cqrs.QueryHandler[query.MintForStudent, model.SignedURLResult]
	Readable       cqrs.QueryHandler[query.Readable, map[string]domain.Kind]
	SignedURL      cqrs.QueryHandler[query.SignedURL, string]
	TotalBytes     cqrs.QueryHandler[query.TotalBytes, int64]
	Usage          cqrs.QueryHandler[query.Usage, domain.Usage]
}

func New(repo domain.Repository, object ports.ObjectStore, probe ports.AudioProbe) *Application {
	service := support.NewService(repo, object, probe)
	return &Application{
		Commands: Commands{
			Delete: command.DeleteHandler{Service: service},
			Update: command.UpdateHandler{Service: service},
			Upload: command.UploadHandler{Service: service},
		},
		Queries: Queries{
			Facets:         query.FacetsHandler{Service: service},
			Get:            query.GetHandler{Service: service},
			List:           query.ListHandler{Service: service},
			MintForStudent: query.MintForStudentHandler{Service: service},
			Readable:       query.ReadableHandler{Service: service},
			SignedURL:      query.SignedURLHandler{Service: service},
			TotalBytes:     query.TotalBytesHandler{Service: service},
			Usage:          query.UsageHandler{Service: service},
		},
		service: service,
	}
}
