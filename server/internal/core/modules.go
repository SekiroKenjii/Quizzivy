package core

import (
	"context"
	"log/slog"

	"quizzivy/internal/api"
	"quizzivy/internal/assignments"
	"quizzivy/internal/auth"
	"quizzivy/internal/auth/google"
	"quizzivy/internal/classes"
	"quizzivy/internal/config"
	"quizzivy/internal/dashboard"
	"quizzivy/internal/db"
	"quizzivy/internal/join"
	"quizzivy/internal/media"
	"quizzivy/internal/questions"
	"quizzivy/internal/storage"
	"quizzivy/internal/students"
	"quizzivy/internal/tests"
	"quizzivy/internal/tests/publish"
)

// buildModules wires every feature module into the handler's dependencies.
//
// The auth service is returned separately because the token-pruning job needs
// it directly, not through the interface the handlers see.
func buildModules(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (api.Deps, *auth.Service, error) {
	boundPasswordHashing(cfg, logger)

	tokens, err := auth.NewTokenIssuer(cfg.JWTSigningKey, cfg.AccessTokenTTL)
	if err != nil {
		return api.Deps{}, nil, err
	}

	authService := auth.NewService(auth.NewStore(pool.Pool), tokens, cfg.RefreshTokenTTL)
	joinService := join.NewService(join.NewStore(pool.Pool))
	attachGoogle(cfg, logger, authService, joinService)

	mediaService, err := newMediaService(ctx, cfg, logger, pool)
	if err != nil {
		return api.Deps{}, nil, err
	}

	deps := api.Deps{
		DB:           pool,
		Auth:         authService,
		Join:         joinService,
		Classes:      classes.NewService(classes.NewStore(pool.Pool)),
		Questions:    questions.NewService(questions.NewStore(pool.Pool)),
		Tests:        tests.NewService(tests.NewStore(pool.Pool)),
		Publisher:    publish.NewPublisher(pool.Pool),
		Dashboard:    dashboard.NewStore(pool.Pool),
		Assignments:  assignments.NewStore(pool.Pool),
		Students:     students.NewStore(pool.Pool),
		Tokens:       tokens,
		RefreshTTL:   cfg.RefreshTokenTTL,
		CookieSecure: cfg.RefreshCookieSecure,
	}
	// Guarded: assigning a nil *media.Service would give the interface a
	// non-nil value, and the handlers' `Deps.Media == nil` check would miss it.
	if mediaService != nil {
		deps.Media = mediaService
	}
	return deps, authService, nil
}

func boundPasswordHashing(cfg config.Config, logger *slog.Logger) {
	auth.SetMaxConcurrentHashes(cfg.MaxConcurrentPasswordHashes)
	logger.Info("password hashing bounded",
		"max_concurrent", cfg.MaxConcurrentPasswordHashes,
		"peak_arena_mib", cfg.MaxConcurrentPasswordHashes*64)
}

// attachGoogle enables §5.3 sign-in when credentials are configured. Config has
// already refused a half-configured set, so this is all-or-nothing.
func attachGoogle(cfg config.Config, logger *slog.Logger, authService *auth.Service, joinService *join.Service) {
	if !cfg.GoogleEnabled() {
		logger.Info("google sign-in disabled (no credentials configured)")
		return
	}

	keys := google.NewKeySet("", nil)
	authService.SetGoogle(google.NewProvider(
		google.NewExchanger(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURIs, "", nil),
		google.NewVerifier(cfg.GoogleClientID, keys),
	), joinService)
	logger.Info("google sign-in enabled", "redirect_uris", cfg.GoogleRedirectURIs)
}

// newMediaService returns nil when object storage is not configured, which is a
// supported deployment: everything but upload still works.
func newMediaService(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (*media.Service, error) {
	if !cfg.MediaEnabled() {
		logger.Info("media storage disabled (no bucket configured)")
		return nil, nil
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
	logger.Info("media storage enabled", "bucket", cfg.S3Bucket, "endpoint", cfg.S3Endpoint)

	return media.NewService(media.NewStore(pool.Pool), objects).
		WithSignedURLTTL(cfg.SignedURLTTL), nil
}
