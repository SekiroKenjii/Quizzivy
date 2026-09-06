package wiring

import (
	"context"
	"log/slog"

	"quizzivy/internal/core/adapters"
	mediaapp "quizzivy/internal/modules/media/application"
	mediahttp "quizzivy/internal/modules/media/http"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/storage"
)

// media returns a nil application when object storage is not configured, which
// is a supported deployment: everything but upload still works.
func media(ctx context.Context, cfg config.Config, logger *slog.Logger, dbx db.Context) (*mediaapp.Application, *mediarepo.Postgres, error) {
	repo := mediarepo.NewPostgres(dbx)
	if !cfg.MediaEnabled() {
		logger.Info("media storage disabled (no bucket configured)")
		return nil, repo, nil
	}

	objects, err := storage.New(ctx, storage.Config{
		Endpoint:        cfg.S3Endpoint,
		Region:          cfg.S3Region,
		Bucket:          cfg.S3Bucket,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		ForcePathStyle:  cfg.S3ForcePathStyle,
	})
	if err != nil {
		return nil, nil, err
	}
	logger.Info("media storage enabled", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)

	return mediaapp.New(repo, objects, adapters.AudioProbe{}).WithSignedURLTTL(cfg.SignedURLTTL), repo, nil
}

func mediaTransport(app *mediaapp.Application) mediahttp.Media {
	return mediahttp.NewMedia(app)
}
