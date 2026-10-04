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

const (
	pruneEvery   = 24 * time.Hour
	pruneTimeout = time.Minute
)

// PruneRefreshTokens deletes refresh-token families whose every token has
// expired.
//
// One machine runs this, so there is nothing to coordinate; a second would
// simply remove nothing, since the DELETE is idempotent. It runs once at
// startup so a long-lived deployment is not the only thing that ever prunes.
func PruneRefreshTokens(ctx context.Context, logger *slog.Logger, svc *identityapp.Application) {
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

	prune()
	ticker := time.NewTicker(pruneEvery)
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
	ticker := time.NewTicker(pruneEvery)
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
