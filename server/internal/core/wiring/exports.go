package wiring

import (
	"context"
	"log/slog"
	"time"

	"quizzivy/internal/core/adapters"
	exportsapp "quizzivy/internal/modules/exports/application"
	exportsworker "quizzivy/internal/modules/exports/application/worker"
	exportshttp "quizzivy/internal/modules/exports/http"
	exportsrepo "quizzivy/internal/modules/exports/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/storage"
)

func exportsStore(ctx context.Context, cfg config.Config) (*storage.Client, error) {
	return storage.New(ctx, storage.Config{
		Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: cfg.ImportBucket,
		AccessKeyID: cfg.S3AccessKeyID, SecretAccessKey: cfg.S3SecretAccessKey, ForcePathStyle: cfg.S3ForcePathStyle,
	})
}

func exports(ctx context.Context, cfg config.Config, logger *slog.Logger, dbx db.Context) (exportshttp.Exports, error) {
	if !cfg.ImportProcessing {
		logger.Info("data export disabled (no import storage and worker configured)")
		return exportshttp.NewExports(nil), nil
	}
	store, err := exportsStore(ctx, cfg)
	if err != nil {
		return exportshttp.Exports{}, err
	}
	wake := adapters.NewWorkerWake(context.WithoutCancel(ctx), cfg.ImportWorkerWakeURL, logger)
	return exportshttp.NewExports(exportsapp.New(exportsrepo.NewPostgres(dbx), store, wake, cfg.AppLocation)), nil
}

// ExportsWorker assembles the runner of the data exports over the worker's
// own database context. Its repository enqueues the requester's ready notice
// into outbox, the worker's mail outbox.
func ExportsWorker(ctx context.Context, cfg config.ImportWorker, dbx db.Context, outbox *email.Outbox, logger *slog.Logger) (exportsworker.Runner, error) {
	store, err := exportsStore(ctx, cfg.Base)
	if err != nil {
		return exportsworker.Runner{}, err
	}
	repo := exportsrepo.NewPostgres(dbx).WithOutbox(outbox)
	return exportsworker.Runner{
		Queue: repo, Source: repo, Store: store, Table: adapters.ExportTable,
		WorkDir: cfg.Base.ImportWorkDir, Timeout: 0, Logger: logger, Now: time.Now,
	}, nil
}
