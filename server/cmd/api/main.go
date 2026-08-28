// Command api serves the Quizzivy backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quizzivy/internal/api"
	"quizzivy/internal/auth"
	"quizzivy/internal/auth/google"
	"quizzivy/internal/config"
	"quizzivy/internal/db"
	"quizzivy/internal/join"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Neon resumes a suspended compute on first connect, so the first ping of a
	// deploy can take seconds. 60s of retries costs nothing on a warm database.
	if err := pool.WaitReady(ctx, 60*time.Second); err != nil {
		return err
	}

	// Before the server starts serving: SetMaxConcurrentHashes is only safe
	// while no handler goroutine exists yet.
	auth.SetMaxConcurrentHashes(cfg.MaxConcurrentPasswordHashes)
	logger.Info("password hashing bounded",
		"max_concurrent", cfg.MaxConcurrentPasswordHashes,
		"peak_arena_mib", cfg.MaxConcurrentPasswordHashes*64)

	tokens, err := auth.NewTokenIssuer(cfg.JWTSigningKey, cfg.AccessTokenTTL)
	if err != nil {
		return err
	}
	authService := auth.NewService(auth.NewStore(pool.Pool), tokens, cfg.RefreshTokenTTL)
	joinService := join.NewService(join.NewStore(pool.Pool))

	if cfg.GoogleEnabled() {
		keys := google.NewKeySet("", nil)
		authService.SetGoogle(google.NewProvider(
			google.NewExchanger(cfg.GoogleClientID, cfg.GoogleClientSecret,
				cfg.GoogleRedirectURIs, "", nil),
			google.NewVerifier(cfg.GoogleClientID, keys),
		), joinService)
		logger.Info("google sign-in enabled", "redirect_uris", cfg.GoogleRedirectURIs)
	} else {
		// Not a warning: a deployment may legitimately run on password login
		// alone. The endpoint reports itself unavailable rather than failing
		// in a way that looks like Google's fault.
		logger.Info("google sign-in disabled (no credentials configured)")
	}

	handler, err := api.NewRouter(api.Deps{
		DB:           pool,
		Auth:         authService,
		Join:         joinService,
		Tokens:       tokens,
		RefreshTTL:   cfg.RefreshTokenTTL,
		CookieSecure: cfg.RefreshCookieSecure,
	}, logger, cfg.AllowedOrigins, cfg.ClientIPHeader)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Long enough for a 10 MB media upload on a slow connection (§11.1).
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	go prunePeriodically(ctx, logger, authService)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "port", cfg.Port, "env", cfg.Env, "origins", cfg.AllowedOrigins)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// prunePeriodically deletes refresh-token families whose every token has
// expired (§5.2's cleanup path).
//
// A single machine runs this, so there is no coordination to do; if a second
// ever runs, the DELETE is idempotent and the loser removes nothing. It runs
// once at startup so a long-lived deployment is not the only thing that ever
// prunes, then daily -- a 30-day token means nothing here is urgent.
func prunePeriodically(ctx context.Context, logger *slog.Logger, svc *auth.Service) {
	const every = 24 * time.Hour

	prune := func() {
		// Its own timeout: a slow prune must not hold anything up, and must not
		// inherit a request deadline it does not have.
		runCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		n, err := svc.PruneExpiredTokens(runCtx)
		if err != nil {
			// Not fatal. Stale rows are inert -- they cannot authenticate
			// anything -- so failing to remove them is untidy, not unsafe.
			logger.Warn("refresh token prune failed", "err", err)
			return
		}
		if n > 0 {
			logger.Info("pruned expired refresh tokens", "rows", n)
		}
	}

	prune()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}
