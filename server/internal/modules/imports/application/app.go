// Package application coordinates Word import use cases: private intake, processing requests,
// teacher review under revision control, and the atomic commit into a draft test.
package application

import (
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/cqrs"
)

type Application struct {
	Commands Commands
	Queries  Queries
}
type Commands struct {
	Create     cqrs.CommandHandler[command.Create, domain.Import]
	Upload     cqrs.CommandHandler[command.Upload, domain.Receipt]
	Process    cqrs.CommandHandler[command.Process, domain.Import]
	Nudge      cqrs.CommandHandler[command.Nudge, cqrs.Nothing]
	Cancel     cqrs.CommandHandler[command.Cancel, domain.Import]
	SaveReview cqrs.CommandHandler[command.SaveReview, domain.ReviewState]
	Adopt      cqrs.CommandHandler[command.Adopt, domain.ReviewState]
	Commit     cqrs.CommandHandler[command.Commit, command.CommitResult]
}
type Queries struct {
	Get          cqrs.QueryHandler[query.Get, domain.Import]
	List         cqrs.QueryHandler[query.List, domain.List]
	Download     cqrs.QueryHandler[query.Download, query.DownloadResult]
	Review       cqrs.QueryHandler[query.Review, domain.ReviewState]
	SourceView   cqrs.QueryHandler[query.SourceView, query.SourceViewResult]
	Limits       cqrs.QueryHandler[query.Limits, query.LimitsResult]
	Capabilities cqrs.QueryHandler[query.Capabilities, query.CapabilitiesResult]
}

// Store is the private import bucket: originals for intake and download, artifacts for the source view.
type Store interface {
	ports.ObjectStore
	ports.ArtifactStore
}

// Dependencies are the ports one import application needs. Legacy admits .doc
// uploads; Processing says a worker is deployed, and without it Process refuses
// rather than queue a run nothing will claim. Worker is told of each queued run.
type Dependencies struct {
	Repo         domain.Repository
	Drafts       domain.Drafts
	Runs         ports.Runs
	Artifacts    domain.Artifacts
	Store        Store
	Inspector    ports.Inspector
	Materializer ports.Materializer
	WorkDir      string
	Quotas       domain.Quotas
	Worker       ports.WorkerSignal
	Legacy       bool
	Processing   bool
}

func New(d Dependencies) *Application {
	reader := worker.EvidenceReader{Artifacts: d.Artifacts, Store: d.Store, WorkDir: d.WorkDir}
	return &Application{
		Commands: Commands{
			Create:     command.CreateHandler{Repo: d.Repo, Quotas: d.Quotas},
			Upload:     command.UploadHandler{Repo: d.Repo, Store: d.Store, Inspector: d.Inspector, WorkDir: d.WorkDir, Quotas: d.Quotas, Slots: make(chan struct{}, 1), Legacy: d.Legacy},
			Process:    command.ProcessHandler{Repo: d.Repo, Runs: d.Runs, Worker: d.Worker, Enabled: d.Processing},
			Nudge:      command.NudgeHandler{Worker: d.Worker},
			Cancel:     command.CancelHandler{Runs: d.Runs},
			SaveReview: command.SaveReviewHandler{Drafts: d.Drafts},
			Adopt:      command.AdoptHandler{Drafts: d.Drafts},
			Commit:     command.CommitHandler{Repo: d.Repo, Drafts: d.Drafts, Materializer: d.Materializer},
		},
		Queries: Queries{
			Get:          query.GetHandler{Repo: d.Repo},
			List:         query.ListHandler{Repo: d.Repo},
			Download:     query.DownloadHandler{Repo: d.Repo, Store: d.Store},
			Review:       query.ReviewHandler{Repo: d.Repo, Drafts: d.Drafts},
			SourceView:   query.SourceViewHandler{Repo: d.Repo, Runs: d.Runs, Reader: reader},
			Limits:       query.LimitsHandler{Legacy: d.Legacy},
			Capabilities: query.CapabilitiesHandler{Processing: d.Processing, Retention: domain.DefaultRetention()},
		},
	}
}
