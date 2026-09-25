package adapters

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// WorkerWake sends the import worker its wake signal. Signals coalesce into one
// pending request, sent in the background with a few short retries, so Wake
// never blocks a request and a burst of queued runs costs one call.
type WorkerWake struct {
	pending chan struct{}
}

// NewWorkerWake starts the sender, which stops with ctx. The API passes a ctx
// that outlives the HTTP drain, so a run queued during shutdown is still announced.
func NewWorkerWake(ctx context.Context, url string, logger *slog.Logger) *WorkerWake {
	w := &WorkerWake{pending: make(chan struct{}, 1)}
	client := &http.Client{Timeout: 2 * time.Second}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.pending:
				if !sendWake(ctx, client, url) {
					logger.Warn("import worker wake failed", "code", "IMPORT_WORKER_WAKE_FAILED")
				}
			}
		}
	}()
	return w
}

// Wake implements ports.WorkerSignal.
func (w *WorkerWake) Wake() {
	select {
	case w.pending <- struct{}{}:
	default:
	}
}

func sendWake(ctx context.Context, client *http.Client, url string) bool {
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return false
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode/100 == 2 {
			return true
		}
	}
	return false
}
