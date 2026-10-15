package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
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
// and the class of a failure (a timeout, a database SQLSTATE, a storage status,
// the network) only: no key, id, address, endpoint, bucket, title or filename,
// which an error from the database or the storage client may carry. A step
// that reports done has had a batch come back short, not necessarily caught
// up.
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
			logger.Warn("housekeeping step failed", "code", "HOUSEKEEPING_STEP_FAILED", "step", step.Name, "removed", removed, "failure", failureClass(err))
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

func failureClass(err error) string {
	var database interface{ SQLState() string }
	var storage interface{ HTTPStatusCode() int }
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &database):
		return "database " + database.SQLState()
	case errors.As(err, &storage) && storage.HTTPStatusCode() > 0:
		return fmt.Sprintf("storage http %d", storage.HTTPStatusCode())
	case errors.As(err, &network):
		return "network"
	}
	return "error"
}
