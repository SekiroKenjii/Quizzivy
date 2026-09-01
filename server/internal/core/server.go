package core

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"quizzivy/internal/config"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	// Long enough for a 10 MB media upload on a slow connection.
	writeTimeout    = 120 * time.Second
	idleTimeout     = 90 * time.Second
	shutdownTimeout = 15 * time.Second
)

// serve runs the HTTP server until ctx is cancelled, then drains in flight
// requests within shutdownTimeout.
func serve(ctx context.Context, logger *slog.Logger, cfg config.Config, handler http.Handler) error {
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
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
		// Background, not ctx: ctx is already cancelled here, and Shutdown would
		// return at once and cut in-flight requests instead of draining them.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
