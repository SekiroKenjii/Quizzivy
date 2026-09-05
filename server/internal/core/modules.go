package core

import (
	"context"
	"errors"
	"log/slog"

	"quizzivy/internal/api"
	"quizzivy/internal/modules/assignments"
	"quizzivy/internal/modules/attempts"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	classesapp "quizzivy/internal/modules/classes/application"
	classeshttp "quizzivy/internal/modules/classes/http"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	dashboardapp "quizzivy/internal/modules/dashboard/application"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	dashboardrepo "quizzivy/internal/modules/dashboard/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	identitydomain "quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/modules/integrity"
	mediaapp "quizzivy/internal/modules/media/application"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/review"
	"quizzivy/internal/modules/tests"
	"quizzivy/internal/modules/tests/publish"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/google"
	"quizzivy/internal/platform/storage"
)

// buildModules wires every feature module into the handler's dependencies.
//
// The auth service is returned separately because the token-pruning job needs
// it directly, not through the interface the handlers see.
func buildModules(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (api.Deps, *identityapp.Service, error) {
	boundPasswordHashing(cfg, logger)

	tokens, err := identityapp.NewTokenIssuer(cfg.JWTSigningKey, cfg.AccessTokenTTL)
	if err != nil {
		return api.Deps{}, nil, err
	}

	authService := identityapp.NewService(identityrepo.NewUsers(pool.Pool), tokens, cfg.RefreshTokenTTL)
	studentStats := attemptsrepo.NewStudentStats(pool.Pool)
	studentsService := identityapp.NewStudents(identityrepo.NewStudents(pool.Pool), studentStats)
	classesRepo := classesrepo.NewPostgres(pool.Pool)
	joinService := classesapp.NewEnrolment(classesRepo)
	attachGoogle(cfg, logger, authService, joinService)

	mediaService, err := newMediaService(ctx, cfg, logger, pool)
	if err != nil {
		return api.Deps{}, nil, err
	}

	deps := api.Deps{
		DB:          pool,
		Tests:       tests.NewService(tests.NewStore(pool.Pool)),
		Publisher:   publish.NewPublisher(pool.Pool),
		Assignments: assignments.NewStore(pool.Pool),
		Attempts:    attempts.NewService(attempts.NewStore(pool.Pool)),
		Review:      review.NewStore(pool.Pool),
		Integrity:   integrity.NewStore(pool.Pool),
		Students:    studentsService,
		Tokens:      tokens,
	}
	if mediaService != nil {
		deps.Media = mediaService
	}
	deps.Modules = api.Modules{
		Dashboard: dashboardhttp.NewDashboard(dashboardapp.New(dashboardrepo.NewPostgres(pool.Pool))),
		Media:     mediahttp.NewMedia(mediaTransport(mediaService)),
		Questions: questionshttp.NewQuestions(questionsapp.NewService(questionsrepo.NewPostgres(pool.Pool), mediaKinds{mediaService}), questionsMedia(mediaService)),
		Identity:  identityhttp.NewIdentity(authService, studentsService, cfg.RefreshTokenTTL, cfg.RefreshCookieSecure),
		Classes:   classeshttp.NewClasses(classesapp.NewService(classesRepo, studentStats), joinService),
	}
	return deps, authService, nil
}

func boundPasswordHashing(cfg config.Config, logger *slog.Logger) {
	identitydomain.SetMaxConcurrentHashes(cfg.MaxConcurrentPasswordHashes)
	logger.Info("password hashing bounded",
		"max_concurrent", cfg.MaxConcurrentPasswordHashes,
		"peak_arena_mib", cfg.MaxConcurrentPasswordHashes*64)
}

// attachGoogle enables §5.3 sign-in when credentials are configured. Config has
// already refused a half-configured set, so this is all-or-nothing.
func attachGoogle(cfg config.Config, logger *slog.Logger, authService *identityapp.Service, joinService *classesapp.Enrolment) {
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
func newMediaService(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (*mediaapp.Service, error) {
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

	return mediaapp.NewService(mediarepo.NewPostgres(pool.Pool), objects).
		WithSignedURLTTL(cfg.SignedURLTTL), nil
}

// mediaKinds lets the question bank ask the media module what an asset is.
type mediaKinds struct{ media *mediaapp.Service }

func (m mediaKinds) Kind(ctx context.Context, assetID string) (string, error) {
	if m.media == nil {
		return "", questionsdomain.ErrMediaNotFound
	}
	asset, err := m.media.Get(ctx, assetID)
	if errors.Is(err, mediadomain.ErrNotFound) {
		return "", questionsdomain.ErrMediaNotFound
	}
	if err != nil {
		return "", err
	}
	return string(asset.Kind), nil
}

func questionsMedia(svc *mediaapp.Service) questionshttp.Media {
	if svc == nil {
		return nil
	}
	return svc
}

func mediaTransport(svc *mediaapp.Service) mediahttp.Service {
	if svc == nil {
		return nil
	}
	return svc
}
