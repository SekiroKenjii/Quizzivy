// Command import-worker processes private Word jobs independently of the HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"quizzivy/internal/core"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := core.RunImportWorker(context.Background(), logger); err != nil {
		logger.Error("import worker stopped", "err", err)
		os.Exit(1)
	}
}
