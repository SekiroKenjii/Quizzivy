package core

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quizzivy/internal/api"
	"quizzivy/internal/auth"
	"quizzivy/internal/config"
	"quizzivy/internal/db"
)

// dbReadyBudget is how long to wait for the database on a cold start. Fly can
// bring the app up before Neon has finished waking.
const dbReadyBudget = 60 * time.Second

// Run is the composition root: load configuration, build every module against a
// live database, then serve until the process is signalled.
//
// cmd/api owns nothing but the logger and the exit code.
func Run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer app.Close()

	return app.Serve(ctx)
}

// App is the assembled application: every module wired to a live pool, behind a
// configured HTTP handler.
type App struct {
	cfg    config.Config
	logger *slog.Logger
	pool   *db.Pool
	auth   *auth.Service
	deps   api.Deps
}

// New opens the database and builds every module. The returned App owns the
// pool, so the caller must Close it.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.WaitReady(ctx, dbReadyBudget); err != nil {
		pool.Close()
		return nil, err
	}

	deps, authService, err := buildModules(ctx, cfg, logger, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return &App{cfg: cfg, logger: logger, pool: pool, auth: authService, deps: deps}, nil
}

func (a *App) Close() {
	a.pool.Close()
}

// Serve starts the background jobs and the HTTP server, and shuts down when ctx
// is cancelled.
func (a *App) Serve(ctx context.Context) error {
	handler, err := api.NewRouter(a.deps, a.logger, a.cfg.AllowedOrigins, a.cfg.ClientIPHeader)
	if err != nil {
		return err
	}

	go prunePeriodically(ctx, a.logger, a.auth)

	return serve(ctx, a.logger, a.cfg, handler)
}
