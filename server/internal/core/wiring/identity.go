package wiring

import (
	"log/slog"

	"quizzivy/internal/core/adapters"
	identityapp "quizzivy/internal/modules/identity/application"
	identityports "quizzivy/internal/modules/identity/application/ports"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	identitydomain "quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/google"
	"quizzivy/internal/shared/stats"
)

func identity(cfg config.Config, logger *slog.Logger, dbx db.Context, stats stats.Source, enroller identityports.SelfEnroller) (*identityapp.Application, *identitytoken.Issuer, error) {
	boundPasswordHashing(cfg, logger)
	tokens, err := identitytoken.NewIssuer(cfg.JWTSigningKey, cfg.AccessTokenTTL)
	if err != nil {
		return nil, nil, err
	}
	app := identityapp.New(identityrepo.NewUsers(dbx), tokens, cfg.RefreshTokenTTL, identityrepo.NewStudents(dbx), stats)
	attachGoogle(cfg, logger, app, enroller)
	return app, tokens, nil
}

func identityTransport(cfg config.Config, app *identityapp.Application, docs *identitytoken.Issuer) identityhttp.Identity {
	return identityhttp.NewIdentity(app, cfg.RefreshTokenTTL, cfg.RefreshCookieSecure, docs)
}

func boundPasswordHashing(cfg config.Config, logger *slog.Logger) {
	identitydomain.Passwords.SetMaxConcurrentHashes(cfg.MaxConcurrentPasswordHashes)
	logger.Info("password hashing bounded",
		"max_concurrent", cfg.MaxConcurrentPasswordHashes,
		"peak_arena_mib", cfg.MaxConcurrentPasswordHashes*64)
}

// attachGoogle enables §5.3 sign-in when credentials are configured. Config has
// already refused a half-configured set, so this is all-or-nothing.
func attachGoogle(cfg config.Config, logger *slog.Logger, app *identityapp.Application, enroller identityports.SelfEnroller) {
	if !cfg.GoogleEnabled() {
		logger.Info("google sign-in disabled (no credentials configured)")
		return
	}

	keys := google.NewKeySet("", nil)
	app.SetGoogle(adapters.NewGoogle(google.NewProvider(
		google.NewExchanger(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURIs, "", nil),
		google.NewVerifier(cfg.GoogleClientID, keys),
	)), enroller)
	logger.Info("google sign-in enabled", "redirect_uris", cfg.GoogleRedirectURIs)
}
