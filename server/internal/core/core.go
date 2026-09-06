package core

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	identityapp "quizzivy/internal/modules/identity/application"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

const dbReadyBudget = 60 * time.Second

// Run is the composition root: load configuration, build every module against a
// live database, then serve until the process is signalled.
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
	auth   *identityapp.Application
	deps   Deps
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
// Handler is the assembled HTTP surface, for the server and for tests that
// drive the whole application in-process.
func (a *App) Handler() (http.Handler, error) {
	return NewRouter(a.deps, a.logger, a.cfg.AllowedOrigins, a.cfg.ClientIPHeader)
}

func (a *App) Serve(ctx context.Context) error {
	handler, err := a.Handler()
	if err != nil {
		return err
	}

	go prunePeriodically(ctx, a.logger, a.auth)

	return Serve(ctx, a.logger, a.cfg, handler)
}
