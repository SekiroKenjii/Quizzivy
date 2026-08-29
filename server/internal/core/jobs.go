package core

import (
	"context"
	"log/slog"
	"time"

	"quizzivy/internal/auth"
)

const (
	pruneEvery   = 24 * time.Hour
	pruneTimeout = time.Minute
)

// prunePeriodically deletes refresh-token families whose every token has
// expired.
//
// One machine runs this, so there is nothing to coordinate; a second would
// simply remove nothing, since the DELETE is idempotent. It runs once at
// startup so a long-lived deployment is not the only thing that ever prunes.
func prunePeriodically(ctx context.Context, logger *slog.Logger, svc *auth.Service) {
	prune := func() {
		runCtx, cancel := context.WithTimeout(ctx, pruneTimeout)
		defer cancel()

		n, err := svc.PruneExpiredTokens(runCtx)
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
