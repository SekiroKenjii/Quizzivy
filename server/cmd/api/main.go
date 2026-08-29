package main

import (
	"context"
	"log/slog"
	"os"

	"quizzivy/internal/core"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := core.Run(context.Background(), logger); err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}
}
