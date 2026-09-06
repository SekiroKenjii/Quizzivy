package core

import (
	"context"
	"log/slog"

	assignmentsapp "quizzivy/internal/modules/assignments/application"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	attemptsapp "quizzivy/internal/modules/attempts/application"
	attemptshttp "quizzivy/internal/modules/attempts/http"
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
	mediaapp "quizzivy/internal/modules/media/application"
	mediahttp "quizzivy/internal/modules/media/http"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionshttp "quizzivy/internal/modules/questions/http"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testshttp "quizzivy/internal/modules/tests/http"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/google"
	"quizzivy/internal/platform/storage"
)

// buildModules wires every feature module into the handler's dependencies.
//
// The auth service is returned separately because the token-pruning job needs
// it directly, not through the interface the handlers see.
func buildModules(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (Deps, *identityapp.Service, error) {
	boundPasswordHashing(cfg, logger)

	tokens, err := identityapp.NewTokenIssuer(cfg.JWTSigningKey, cfg.AccessTokenTTL)
	if err != nil {
		return Deps{}, nil, err
	}

	authService := identityapp.NewService(identityrepo.NewUsers(db.NewContext(pool.Pool)), tokens, cfg.RefreshTokenTTL)
	studentStats := attemptsrepo.NewStudentStats(db.NewContext(pool.Pool))
	studentsService := identityapp.NewStudents(identityrepo.NewStudents(db.NewContext(pool.Pool)), studentStats)
	classesRepo := classesrepo.NewPostgres(db.NewContext(pool.Pool))
	classesApp := classesapp.New(classesRepo, studentStats)
	attachGoogle(cfg, logger, authService, selfEnroller{classesApp})

	mediaRepo := mediarepo.NewPostgres(db.NewContext(pool.Pool))
	questionsRepo := questionsrepo.NewPostgres(db.NewContext(pool.Pool))
	testsRepo := testsrepo.NewPostgres(db.NewContext(pool.Pool), questionsRepo, mediaRepo)
	mediaService, err := newMediaService(ctx, cfg, logger, mediaRepo)
	if err != nil {
		return Deps{}, nil, err
	}

	deps := Deps{
		DB:     pool,
		Tokens: tokens,
	}
	deps.Modules = Modules{
		Dashboard:   dashboardhttp.NewDashboard(dashboardapp.New(dashboardrepo.NewPostgres(db.NewContext(pool.Pool)))),
		Media:       mediahttp.NewMedia(mediaTransport(mediaService)),
		Attempts:    attemptshttp.NewAttempts(attemptsapp.NewService(attemptsrepo.NewPostgres(db.NewContext(pool.Pool))), attemptsapp.NewReview(attemptsrepo.NewReviews(db.NewContext(pool.Pool))), attemptsapp.NewIntegrity(attemptsrepo.NewTimelines(db.NewContext(pool.Pool))), attemptsMedia(mediaService), studentsService, logger),
		Assignments: assignmentshttp.NewAssignments(assignmentsapp.New(assignmentsrepo.NewPostgres(db.NewContext(pool.Pool)))),
		Tests:       testshttp.NewTests(testsapp.NewService(testsRepo), testsapp.NewPublisher(testsRepo), testsMedia(mediaService)),
		Questions:   questionshttp.NewQuestions(questionsapp.NewService(questionsRepo, mediaKinds{mediaService}), questionsMedia(mediaService)),
		Identity:    identityhttp.NewIdentity(authService, studentsService, cfg.RefreshTokenTTL, cfg.RefreshCookieSecure),
		Classes:     classeshttp.NewClasses(classesApp),
	}
	return deps, authService, nil
}

func boundPasswordHashing(cfg config.Config, logger *slog.Logger) {
	identitydomain.Passwords.SetMaxConcurrentHashes(cfg.MaxConcurrentPasswordHashes)
	logger.Info("password hashing bounded",
		"max_concurrent", cfg.MaxConcurrentPasswordHashes,
		"peak_arena_mib", cfg.MaxConcurrentPasswordHashes*64)
}

// attachGoogle enables §5.3 sign-in when credentials are configured. Config has
// already refused a half-configured set, so this is all-or-nothing.
func attachGoogle(cfg config.Config, logger *slog.Logger, authService *identityapp.Service, enroller identityapp.SelfEnroller) {
	if !cfg.GoogleEnabled() {
		logger.Info("google sign-in disabled (no credentials configured)")
		return
	}

	keys := google.NewKeySet("", nil)
	authService.SetGoogle(googleProvider{google.NewProvider(
		google.NewExchanger(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURIs, "", nil),
		google.NewVerifier(cfg.GoogleClientID, keys),
	)}, enroller)
	logger.Info("google sign-in enabled", "redirect_uris", cfg.GoogleRedirectURIs)
}

// newMediaService returns nil when object storage is not configured, which is a
// supported deployment: everything but upload still works.
func newMediaService(ctx context.Context, cfg config.Config, logger *slog.Logger, repo *mediarepo.Postgres) (*mediaapp.Service, error) {
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

	return mediaapp.NewService(repo, objects, audioProbe{}).
		WithSignedURLTTL(cfg.SignedURLTTL), nil
}
