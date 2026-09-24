// Package application coordinates private Word source intake and authorized source access.
package application

import (
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/cqrs"
)

type Application struct {
	Commands Commands
	Queries  Queries
}
type Commands struct {
	Create cqrs.CommandHandler[command.Create, domain.Import]
	Upload cqrs.CommandHandler[command.Upload, domain.Receipt]
}
type Queries struct {
	Get      cqrs.QueryHandler[query.Get, domain.Import]
	List     cqrs.QueryHandler[query.List, domain.List]
	Download cqrs.QueryHandler[query.Download, query.DownloadResult]
}

func New(repo domain.Repository, store ports.ObjectStore, inspector ports.Inspector, workDir string, quotas domain.Quotas) *Application {
	return &Application{
		Commands: Commands{Create: command.CreateHandler{Repo: repo, Quotas: quotas}, Upload: command.UploadHandler{Repo: repo, Store: store, Inspector: inspector, WorkDir: workDir, Quotas: quotas, Slots: make(chan struct{}, 1)}},
		Queries:  Queries{Get: query.GetHandler{Repo: repo}, List: query.ListHandler{Repo: repo}, Download: query.DownloadHandler{Repo: repo, Store: store}},
	}
}
