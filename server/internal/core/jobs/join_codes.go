package jobs

import (
	"context"
	"log/slog"
	"time"

	classesapp "quizzivy/internal/modules/classes/application"
	classescommand "quizzivy/internal/modules/classes/application/command"
)

const legacyRotationBudget = 2 * time.Minute

// RotateLegacyJoinCodes replaces, once, every join code only a SHA-256 holds
// and tells each class's teacher, under a two-minute budget, and returns: it
// never runs on a timer, and a class it leaves behind waits for the next
// start-up. Each class is rotated in its own transaction under an advisory
// lock, so a second machine starting at the same time rotates no class twice.
// It logs legacy_join_codes_found before the first class when there is one,
// legacy_join_codes_rotated with the run's counts after the last, and a
// warning for each class left unrotated and each teacher left untold; a run
// that finds nothing logs nothing. No line carries a code or a hint.
func RotateLegacyJoinCodes(ctx context.Context, logger *slog.Logger, app *classesapp.Application) {
	runCtx, cancel := context.WithTimeout(ctx, legacyRotationBudget)
	defer cancel()

	run, err := app.Commands.RotateLegacyJoinCodes.Handle(runCtx, classescommand.RotateLegacyJoinCodes{
		Found: func(count int) {
			if count > 0 {
				logger.Info("legacy_join_codes_found", "count", count)
			}
		},
		Problem: func(err error) {
			logger.Warn("legacy_join_codes_problem", "err", err)
		},
	})
	if err != nil {
		logger.Warn("legacy_join_codes_failed", "err", err)
		return
	}
	if run.Found == 0 {
		return
	}
	logger.Info("legacy_join_codes_rotated",
		"found", run.Found, "rotated", run.Rotated, "failed", run.Failed,
		"teachers", run.Teachers, "notify_failed", run.NotifyFailed)
}
