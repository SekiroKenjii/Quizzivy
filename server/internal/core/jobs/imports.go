package jobs

import (
	"context"
	"errors"
	"log/slog"
	"quizzivy/internal/modules/imports/application/worker"
	"time"
)

// ImportRunner executes at most one claimed job and returns promptly when idle or cancelled.
type ImportRunner interface {
	RunOne(context.Context) (bool, error)
}

// RunImports polls serially and drains the active claim on cancellation; persistent leases recover crashes between polls.
func RunImports(ctx context.Context, logger *slog.Logger, runner ImportRunner, poll time.Duration) error {
	if poll <= 0 {
		return errors.New("import worker poll interval must be positive")
	}
	for ctx.Err() == nil {
		worked, err := runner.RunOne(ctx)
		if err != nil {
			logger.Warn("import worker poll failed", "code", "WORKER_POLL_FAILED")
		}
		if worked && err == nil {
			continue
		}
		timer := time.NewTimer(poll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	return nil
}

// ObserveImportRun logs operational identities, outcome codes and timing without source text, object keys or signed URLs.
func ObserveImportRun(logger *slog.Logger) func(worker.Event) {
	return func(e worker.Event) {
		logger.Info("import worker event", "kind", e.Kind, "runId", e.RunID, "importId", e.ImportID, "stage", e.Stage, "attempt", e.Attempt, "code", e.Code, "durationMs", e.Duration.Milliseconds())
	}
}
