package jobs

import (
	"context"
	"log/slog"
	"time"

	aiapp "quizzivy/internal/modules/ai/application"
	aicommand "quizzivy/internal/modules/ai/application/command"
)

const (
	aiReservationsEvery   = time.Hour
	aiReservationsTimeout = time.Minute
)

// ReleaseStaleAiReservations returns the credits of AI calls that never finished, once at start-up and then hourly, each run under its own time budget. A reservation is held for the one call that made it and settled or released within the provider's budget of two minutes, so one older than ten is the leftover of a process that died, and its credits are returned to the pool and the teacher. The release is idempotent, so a second machine or a second run returns nothing, and a failed run is logged and left to the next. It is started only when the provider is configured, because nothing is reserved without it.
func ReleaseStaleAiReservations(ctx context.Context, logger *slog.Logger, app *aiapp.Application) {
	release := func() {
		runCtx, cancel := context.WithTimeout(ctx, aiReservationsTimeout)
		defer cancel()

		released, err := app.Commands.ReleaseStale.Handle(runCtx, aicommand.ReleaseStale{})
		if err != nil {
			logger.Warn("ai reservation sweep failed", "err", err)
			return
		}
		if released.Reservations > 0 {
			logger.Info("released stale ai reservations", "reservations", released.Reservations, "credits", released.Credits)
		}
	}

	release()
	ticker := time.NewTicker(aiReservationsEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			release()
		}
	}
}
