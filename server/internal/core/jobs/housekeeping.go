package jobs

import (
	"context"
	"log/slog"
	"time"
)

const (
	housekeepingBatch  = 500
	housekeepingRounds = 20
	housekeepingBudget = time.Minute
)

// HousekeepingStep is one bounded piece of the daily housekeeping. Batch
// removes at most limit rows or objects and reports how many it removed; a
// count below limit means the step has caught up.
type HousekeepingStep struct {
	Name  string
	Batch func(ctx context.Context, limit int) (int, error)
}

// Housekeep runs each step once, in order, under a budget of its own: it
// repeats a step's batches while they come back full, at most
// housekeepingRounds times, and stops the step at its first error or when its
// budget ends. A step that fails or runs out of time is logged and does not
// stop the others, and the next run picks up where it stopped. It logs counts
// only: no key, address, title or filename.
func Housekeep(ctx context.Context, logger *slog.Logger, steps []HousekeepingStep) {
	for _, step := range steps {
		if ctx.Err() != nil {
			return
		}
		runStep(ctx, logger, step)
	}
}

func runStep(ctx context.Context, logger *slog.Logger, step HousekeepingStep) {
	run, cancel := context.WithTimeout(ctx, housekeepingBudget)
	defer cancel()

	removed, more := 0, false
	for range housekeepingRounds {
		n, err := step.Batch(run, housekeepingBatch)
		removed += n
		if err != nil {
			logger.Warn("housekeeping step failed", "code", "HOUSEKEEPING_STEP_FAILED", "step", step.Name, "removed", removed, "err", err)
			return
		}
		more = n >= housekeepingBatch
		if !more {
			break
		}
	}
	if more {
		logger.Info("housekeeping step left work for the next run", "step", step.Name, "removed", removed)
		return
	}
	logger.Info("housekeeping step done", "step", step.Name, "removed", removed)
}
