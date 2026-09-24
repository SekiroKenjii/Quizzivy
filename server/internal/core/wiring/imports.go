package wiring

import (
	"context"
	"fmt"
	"os"
	"quizzivy/internal/core/adapters"
	importsapp "quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	importsrepo "quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/storage"
)

func imports(ctx context.Context, cfg config.Config, dbx db.Context) (importshttp.Imports, error) {
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
	return importshttp.New(importsapp.New(importsrepo.NewPostgres(dbx), store, adapters.ImportInspector{}, cfg.ImportWorkDir, quotas)), nil
}
