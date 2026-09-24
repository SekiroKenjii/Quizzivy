package wiring

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"os"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/storage"
	"quizzivy/internal/platform/wordconvert"
	"time"
)

// ImportWorker assembles a single synchronous pipeline, with database leases bounding concurrency across processes.
func ImportWorker(ctx context.Context, cfg config.ImportWorker, dbx db.Context) (worker.Runner, error) {
	base := cfg.Base
	if err := os.MkdirAll(base.ImportWorkDir, 0o700); err != nil {
		return worker.Runner{}, err
	}
	converter, err := wordconvert.New(base.ImportWorkDir, cfg.DockerBinary, cfg.ImageID, 2*time.Minute)
	if err != nil {
		return worker.Runner{}, fmt.Errorf("prepare isolated import converter: %w", err)
	}
	store, err := storage.New(ctx, storage.Config{Endpoint: base.S3Endpoint, Region: base.S3Region, Bucket: base.ImportBucket, AccessKeyID: base.S3AccessKeyID, SecretAccessKey: base.S3SecretAccessKey, ForcePathStyle: base.S3ForcePathStyle})
	if err != nil {
		return worker.Runner{}, err
	}
	repo := repositories.NewPostgres(dbx)
	pipeline := worker.Pipeline{Sources: repo, Artifacts: repo, Store: store, Engine: adapters.ImportProcessing{Converter: converter, ImageID: cfg.ImageID, WorkDir: base.ImportWorkDir}, WorkDir: base.ImportWorkDir,
		Quotas: domain.ArtifactQuotas{ActorBytes: int64(cfg.ActorArtifactMiB) << 20, GlobalBytes: int64(cfg.GlobalArtifactMiB) << 20, SetsPerImport: cfg.SetsPerImport}}
	return worker.Runner{Queue: repo, Processor: pipeline, Policy: domain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, Lease: time.Minute, GlobalLimit: cfg.GlobalWorkers, ActorLimit: cfg.ActorWorkers}, HeartbeatEvery: 5 * time.Second, Timeout: 5 * time.Minute, RetryAfter: 2 * time.Minute}, nil
}
