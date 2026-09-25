package jobs

import (
	"context"
	"log/slog"
	"time"

	"quizzivy/internal/modules/imports/application/worker"
)

// ImportRetention is how often the API sweeps import files, how soon it
// retries a failed sweep, and how long one sweep may run.
type ImportRetention struct {
	Every, Retry, Budget time.Duration
}

// SweepImportFiles applies the import retention policy at start-up and then
// on the schedule, retrying sooner after a failed sweep. It runs in the API,
// which exists wherever import storage does, and logs counts only: no titles,
// filenames or object keys.
func SweepImportFiles(ctx context.Context, logger *slog.Logger, sweep func(context.Context, time.Time) (worker.Swept, error), schedule ImportRetention) {
	for {
		run, cancel := context.WithTimeout(ctx, schedule.Budget)
		swept, err := sweep(run, time.Now())
		cancel()
		next := schedule.Every
		switch {
		case err != nil:
			next = schedule.Retry
			logger.Warn("import retention sweep failed", "code", "IMPORT_RETENTION_FAILED", "closed", swept.Closed, "removed", swept.Removed, "failed", swept.Failed)
		case swept.Failed > 0:
			logger.Warn("import retention left files in place", "code", "IMPORT_RETENTION_INCOMPLETE", "closed", swept.Closed, "removed", swept.Removed, "failed", swept.Failed)
		default:
			logger.Info("import retention swept", "closed", swept.Closed, "removed", swept.Removed)
		}
		timer := time.NewTimer(next)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
