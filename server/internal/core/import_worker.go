package core

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"quizzivy/internal/core/jobs"
	"quizzivy/internal/core/wiring"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"runtime/debug"
	"syscall"
	"time"
)

// RunImportWorker starts a separate processor with a conservative Go heap target; Docker conversion has its own independent hard limits.
func RunImportWorker(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.LoadImportWorker()
	if err != nil {
		return err
	}
	previous := debug.SetMemoryLimit(-1)
	if previous > 512<<20 {
		debug.SetMemoryLimit(512 << 20)
		defer debug.SetMemoryLimit(previous)
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := db.Open(ctx, cfg.Base.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.WaitReady(ctx, dbReadyBudget); err != nil {
		return err
	}
	runner, err := wiring.ImportWorker(ctx, cfg, db.NewContext(pool.Pool))
	if err != nil {
		return err
	}
	runner.Observe = jobs.ObserveImportRun(logger)
	wake := make(chan struct{}, 1)
	if err := jobs.ServeWake(ctx, logger, cfg.WakeAddress, wake); err != nil {
		return err
	}
	logger.Info("import worker started", "pipeline", runner.Policy.PipelineVersion, "workerId", runner.Policy.WorkerID)
	return jobs.RunImports(ctx, logger, runner, jobs.ImportSchedule{Wake: wake, Idle: cfg.IdlePoll, Retry: 2 * time.Second})
}
