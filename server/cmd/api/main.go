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
	"quizzivy/internal/config"
	"quizzivy/internal/db"
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

	tokens, err := auth.NewTokenIssuer(cfg.JWTSigningKey, cfg.AccessTokenTTL)
	if err != nil {
		return err
	}
	authService := auth.NewService(auth.NewStore(pool.Pool), tokens, cfg.RefreshTokenTTL)

	handler, err := api.NewRouter(api.Deps{
		DB:           pool,
		Auth:         authService,
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
