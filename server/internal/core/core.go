// Package core is the composition root: it opens the database, has wiring assemble every module, puts the router in front of them and runs the server and the background jobs for the life of the process.
package core

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quizzivy/internal/core/jobs"
	"quizzivy/internal/core/router"
	"quizzivy/internal/core/wiring"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/httpserver"
)

const dbReadyBudget = 60 * time.Second

// Run loads configuration, builds every module against a live database, then
// serves until the process is signalled.
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
	cfg      config.Config
	logger   *slog.Logger
	pool     *db.Pool
	assembly wiring.Assembly
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

	assembly, err := wiring.Build(ctx, cfg, logger, pool)
	if err != nil {
		pool.Close()
		return nil, err
	}

	return &App{cfg: cfg, logger: logger, pool: pool, assembly: assembly}, nil
}

func (a *App) Close() {
	a.pool.Close()
}

// Handler is the assembled HTTP surface, for the server and for tests that
// drive the whole application in-process.
func (a *App) Handler() (http.Handler, error) {
	deps := router.Deps{Modules: a.assembly.Modules, DB: a.pool, Tokens: a.assembly.Tokens, Docs: a.assembly.Docs, DocsPublic: a.cfg.DocsPublic}
	return router.New(deps, a.logger, a.cfg.AllowedOrigins, a.cfg.ClientIPHeader)
}

// Serve starts the background jobs and the HTTP server, and shuts down when ctx
// is cancelled.
func (a *App) Serve(ctx context.Context) error {
	handler, err := a.Handler()
	if err != nil {
		return err
	}

	go jobs.PruneRefreshTokens(ctx, a.logger, a.assembly.Identity)

	return httpserver.Serve(ctx, a.logger, a.cfg, handler)
}
