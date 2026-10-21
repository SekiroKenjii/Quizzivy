// Package jobs schedules the background commands the process runs on its own clock.
package jobs

import (
	"context"
	"log/slog"
	identitycommand "quizzivy/internal/modules/identity/application/command"
	"time"

	identityapp "quizzivy/internal/modules/identity/application"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
)

// PruneInterval is how often the daily jobs run after their start-up run.
const PruneInterval = 24 * time.Hour

const pruneTimeout = time.Minute

// PruneRefreshTokens deletes refresh-token families whose every token has
// expired, once at start-up and then daily, and runs each of daily after every
// prune: the housekeeping rides this tick, so it adds no wake of its own.
//
// One machine runs this, so there is nothing to coordinate; a second would
// simply remove nothing, since the DELETE is idempotent. It runs once at
// startup so a long-lived deployment is not the only thing that ever prunes.
func PruneRefreshTokens(ctx context.Context, logger *slog.Logger, svc *identityapp.Application, daily ...func(context.Context)) {
	PruneRefreshTokensEvery(ctx, logger, svc, PruneInterval, daily...)
}

// PruneRefreshTokensEvery is PruneRefreshTokens on a tick of every, which
// PruneInterval is for the process and a test shortens.
func PruneRefreshTokensEvery(ctx context.Context, logger *slog.Logger, svc *identityapp.Application, every time.Duration, daily ...func(context.Context)) {
	prune := func() {
		runCtx, cancel := context.WithTimeout(ctx, pruneTimeout)
		defer cancel()

		n, err := svc.Commands.PruneExpiredTokens.Handle(runCtx, identitycommand.PruneExpiredTokens{})
		if err != nil {
			logger.Warn("refresh token prune failed", "err", err)
			return
		}
		if n > 0 {
			logger.Info("pruned expired refresh tokens", "rows", n)
		}
	}
	tick := func() {
		prune()
		for _, run := range daily {
			run(ctx)
		}
	}

	tick()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

// PruneNotifications deletes the notifications past their retention, once at
// start-up and then daily, each run under its own time budget. The delete is
// idempotent, so a second machine or a second run removes nothing, and a
// failed run is logged and left to the next one.
func PruneNotifications(ctx context.Context, logger *slog.Logger, app *notificationsapp.Application) {
	prune := func() {
		runCtx, cancel := context.WithTimeout(ctx, pruneTimeout)
		defer cancel()

		n, err := app.Commands.Prune.Handle(runCtx, notificationscommand.Prune{})
		if err != nil {
			logger.Warn("notification prune failed", "err", err)
			return
		}
		if n > 0 {
			logger.Info("pruned old notifications", "rows", n)
		}
	}

	prune()
	ticker := time.NewTicker(PruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prune()
		}
	}
}
