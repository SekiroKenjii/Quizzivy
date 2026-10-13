package wiring

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log/slog"
	"quizzivy/internal/core/adapters"
	aiapp "quizzivy/internal/modules/ai/application"
	airepo "quizzivy/internal/modules/ai/repositories"
	"quizzivy/internal/modules/imports/application/reader"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/storage"
	"quizzivy/internal/platform/wordconvert"
	"time"
)

// ImportWorker assembles a single synchronous pipeline, with database leases bounding concurrency across processes.
func ImportWorker(ctx context.Context, cfg config.ImportWorker, dbx db.Context, logger *slog.Logger) (worker.Runner, error) {
	base := cfg.Base
	if err := PrivateWorkDir(base.ImportWorkDir); err != nil {
		return worker.Runner{}, err
	}
	var converter *wordconvert.Converter
	if cfg.ImageID != "" {
		var err error
		converter, err = wordconvert.New(base.ImportWorkDir, cfg.DockerBinary, cfg.ImageID, 2*time.Minute)
		if err != nil {
			return worker.Runner{}, fmt.Errorf("prepare isolated import converter: %w", err)
		}
	}
	store, err := storage.New(ctx, storage.Config{Endpoint: base.S3Endpoint, Region: base.S3Region, Bucket: base.ImportBucket, AccessKeyID: base.S3AccessKeyID, SecretAccessKey: base.S3SecretAccessKey, ForcePathStyle: base.S3ForcePathStyle})
	if err != nil {
		return worker.Runner{}, err
	}
	repo := repositories.NewPostgres(dbx)
	pdf := &pdftext.Reader{}
	scans, err := scanReader(cfg.Base, dbx, logger, pdf)
	if err != nil {
		return worker.Runner{}, err
	}
	timeout := 5 * time.Minute
	if scans != nil {
		timeout = aiRunTimeout
	}
	pipeline := worker.Pipeline{Sources: repo, Artifacts: repo, Store: store, Engine: adapters.ImportProcessing{Converter: converter, PDF: pdf, AI: scans, Logger: logger, ImageID: cfg.ImageID, WorkDir: base.ImportWorkDir}, WorkDir: base.ImportWorkDir,
		Quotas: domain.ArtifactQuotas{ActorBytes: int64(cfg.ActorArtifactMiB) << 20, GlobalBytes: int64(cfg.GlobalArtifactMiB) << 20, SetsPerImport: cfg.SetsPerImport}}
	return worker.Runner{Queue: repo, Processor: pipeline, Policy: domain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, Lease: time.Minute, GlobalLimit: cfg.GlobalWorkers, ActorLimit: cfg.ActorWorkers}, HeartbeatEvery: 5 * time.Second, Timeout: timeout, RetryAfter: 2 * time.Minute}, nil
}

const aiRunTimeout = 15 * time.Minute

// scanReader builds the AI reader of scanned PDFs over the worker's own database context, or returns nil when no provider is configured, in which case a scan is refused as it was before the assistant. It meters through the same credits model the API does, and tells a teacher who crosses the allowance's warning share as the API would.
func scanReader(cfg config.Config, dbx db.Context, logger *slog.Logger, pdf *pdftext.Reader) (*reader.Reader, error) {
	completer, err := adapters.NewAI(cfg.AI, logger)
	if err != nil || completer == nil {
		return nil, err
	}
	zone, err := config.AppLocation()
	if err != nil {
		return nil, err
	}
	credits := aiapp.New(airepo.NewPostgres(dbx), aiapp.Options{Prices: aiPrices(cfg.AI), Zone: zone, Notifier: notifications(dbx).Commands.Notify, Logger: logger})
	return &reader.Reader{Pages: adapters.PDFPages{PDF: pdf}, Reserve: credits.Commands.Reserve, Finish: credits.Commands.Finish, Completer: completer, Logger: logger}, nil
}
