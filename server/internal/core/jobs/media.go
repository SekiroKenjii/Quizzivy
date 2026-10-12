package jobs

import (
	"context"
	"log/slog"
	"time"

	mediacommand "quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/shared/cqrs"
)

// MediaRetention is how often the API sweeps the media files that have left a
// library, how soon it retries a failed sweep, and how long one sweep may run.
type MediaRetention struct {
	Every, Retry, Budget time.Duration
}

// SweepRetiredMedia removes the stored object of every replaced or deleted
// media file seven days after nothing names it, at start-up and then on the
// schedule, retrying sooner after a failed sweep. Every API machine runs it: a
// file is taken under a row lock that a second sweep skips, so none is removed
// twice. It logs counts and the first failure's error, which names an object
// key of no title or filename.
func SweepRetiredMedia(ctx context.Context, logger *slog.Logger, sweep cqrs.CommandHandler[mediacommand.Sweep, mediacommand.Swept], schedule MediaRetention) {
	runEvery(ctx, func() time.Duration {
		run, cancel := context.WithTimeout(ctx, schedule.Budget)
		swept, err := sweep.Handle(run, mediacommand.Sweep{})
		cancel()
		switch {
		case err != nil:
			logger.Warn("media retention sweep failed", "code", "MEDIA_RETENTION_FAILED", "err", err, "unreferenced", swept.Unreferenced, "cancelled", swept.Cancelled, "removed", swept.Removed, "failed", swept.Failed)
			return schedule.Retry
		case swept.Failed > 0:
			logger.Warn("media retention left files in place", "code", "MEDIA_RETENTION_INCOMPLETE", "err", swept.Cause, "unreferenced", swept.Unreferenced, "cancelled", swept.Cancelled, "removed", swept.Removed, "failed", swept.Failed)
			return schedule.Retry
		default:
			logger.Info("media retention swept", "unreferenced", swept.Unreferenced, "cancelled", swept.Cancelled, "removed", swept.Removed)
			return schedule.Every
		}
	})
}
