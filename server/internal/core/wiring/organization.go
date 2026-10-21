package wiring

import (
	"context"
	"log/slog"

	"quizzivy/internal/core/adapters"
	organizationapp "quizzivy/internal/modules/organization/application"
	organizationhttp "quizzivy/internal/modules/organization/http"
	organizationrepo "quizzivy/internal/modules/organization/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/imagesafe"
	"quizzivy/internal/platform/storage"
)

func organization(ctx context.Context, cfg config.Config, logger *slog.Logger, dbx db.Context, images *imagesafe.Processor) (*organizationapp.Application, error) {
	app := organizationapp.New(organizationrepo.NewPostgres(dbx), logger, cfg.GoogleEnabled())
	if !cfg.MediaEnabled() {
		logger.Info("organization logos disabled (no bucket configured)")
		return app, nil
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
		return nil, err
	}
	logger.Info("organization logos enabled", "bucket", cfg.S3Bucket)
	return app.WithLogos(adapters.LogoStore{Objects: objects}, adapters.LogoChecker{Images: images}), nil
}

// OrganizationBranding builds the branding read of a process that does not
// assemble the organization module, such as the import worker. It refreshes
// as the module's snapshot does, at most every 30 seconds.
func OrganizationBranding(cfg config.Config, dbx db.Context, logger *slog.Logger) Branding {
	return organizationapp.New(organizationrepo.NewPostgres(dbx), logger, cfg.GoogleEnabled()).Queries.Branding
}

func organizationTransport(app *organizationapp.Application, cfg config.Config) organizationhttp.Organization {
	return organizationhttp.NewOrganization(app, cfg.Limits)
}
