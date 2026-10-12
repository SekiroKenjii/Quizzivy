package cqrs

import (
	"context"
	"log/slog"
	"time"
)

// AnnounceBudget is how long one Announce may take, reading what it will say
// and delivering all of it.
const AnnounceBudget = 10 * time.Second

// Announce tells another module what a committed change brought about. It
// runs build, which returns the commands to deliver, and hands each to h in
// order, all under AnnounceBudget and a context that keeps ctx's values and
// not its cancellation, so a request that ended after its commit still tells.
//
// Telling never fails the change it follows: a failure of build or of a
// delivery is logged, once, with what names the announcement and how many
// commands were not delivered, and is not returned. Delivery stops at the
// first failure that leaves the budget spent. A nil h announces nothing and
// does not call build, so a module run without the other one reads nothing
// for it.
func Announce[C any](ctx context.Context, h CommandHandler[C, Nothing], logger *slog.Logger, what string, build func(context.Context) ([]C, error)) {
	AnnounceWithin(ctx, AnnounceBudget, h, logger, what, build)
}

// AnnounceWithin is Announce under a budget the caller names.
func AnnounceWithin[C any](ctx context.Context, budget time.Duration, h CommandHandler[C, Nothing], logger *slog.Logger, what string, build func(context.Context) ([]C, error)) {
	if h == nil {
		return
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
	defer cancel()

	commands, err := build(ctx)
	if err != nil {
		logger.WarnContext(ctx, "announcement not prepared", "what", what, "error", err)
		return
	}
	undelivered := 0
	var first error
	for i, c := range commands {
		if _, err := h.Handle(ctx, c); err != nil {
			undelivered++
			if first == nil {
				first = err
			}
			if ctx.Err() != nil {
				undelivered += len(commands) - i - 1
				break
			}
		}
	}
	if undelivered > 0 {
		logger.WarnContext(ctx, "announcement not delivered", "what", what, "commands", len(commands), "undelivered", undelivered, "error", first)
	}
}
