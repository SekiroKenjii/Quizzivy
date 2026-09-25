package wiring

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"quizzivy/internal/core/adapters"
	importsapp "quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	importsrepo "quizzivy/internal/modules/imports/repositories"
	mediaapp "quizzivy/internal/modules/media/application"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/storage"
)

func imports(ctx context.Context, cfg config.Config, logger *slog.Logger, dbx db.Context, mediaApp *mediaapp.Application) (importshttp.Imports, error) {
	if cfg.ImportBucket == "" {
		return importshttp.New(nil), nil
	}
	if err := os.MkdirAll(cfg.ImportWorkDir, 0700); err != nil {
		return importshttp.Imports{}, fmt.Errorf("imports: prepare private work directory: %w", err)
	}
	info, err := os.Lstat(cfg.ImportWorkDir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return importshttp.Imports{}, fmt.Errorf("IMPORT_WORK_DIR must be a private directory with mode 0700")
	}
	store, err := storage.New(ctx, storage.Config{Endpoint: cfg.S3Endpoint, Region: cfg.S3Region, Bucket: cfg.ImportBucket, AccessKeyID: cfg.S3AccessKeyID, SecretAccessKey: cfg.S3SecretAccessKey, ForcePathStyle: cfg.S3ForcePathStyle})
	if err != nil {
		return importshttp.Imports{}, err
	}
	quotas := domain.Quotas{ActorImports: cfg.ImportActorCount, GlobalImports: cfg.ImportGlobalCount, SourcesPerImport: cfg.ImportSourcesPerItem, ActorBytes: int64(cfg.ImportActorMiB) << 20, GlobalBytes: int64(cfg.ImportGlobalMiB) << 20}
	repo := importsrepo.NewPostgres(dbx)
	committer := adapters.ImportCommitter{
		DB: dbx,
		Tests: func(scoped db.Context) *testsapp.Application {
			questionsRepo, mediaRepo := questionsrepo.NewPostgres(scoped), mediarepo.NewPostgres(scoped)
			return tests(scoped, questionsRepo, mediaRepo, mediaApp)
		},
		Questions: func(scoped db.Context) *questionsapp.Application {
			app, _ := questions(scoped, mediaApp)
			return app
		},
	}
	deps := importsapp.Dependencies{Repo: repo, Drafts: repo, Runs: repo, Artifacts: repo, Store: store, Inspector: adapters.ImportInspector{}, Materializer: committer, WorkDir: cfg.ImportWorkDir, Quotas: quotas, Legacy: cfg.ImportLegacyDoc, Processing: cfg.ImportProcessing}
	if cfg.ImportProcessing {
		deps.Worker = adapters.NewWorkerWake(context.WithoutCancel(ctx), cfg.ImportWorkerWakeURL, logger)
	}
	return importshttp.New(importsapp.New(deps)), nil
}
