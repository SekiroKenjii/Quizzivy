package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"quizzivy/internal/modules/imports/application/worker"
	"time"
)

// ImportRunner executes at most one claimed job and returns promptly when idle
// or cancelled, and reports when queued work next falls due.
type ImportRunner interface {
	RunOne(context.Context) (bool, error)
	NextDue(context.Context) (time.Time, bool, error)
}

// ImportSchedule decides how long an idle worker sleeps. It wakes on a Wake
// signal, when queued work falls due, or after Idle at the latest, so an idle
// worker does not query the database on a short timer. It never polls sooner
// than Retry, which is also the pause after a failed poll.
type ImportSchedule struct {
	Wake        <-chan struct{}
	Idle, Retry time.Duration
}

// RunImports claims serially until the queue is empty, then sleeps as the
// schedule allows; persistent leases recover crashes between polls.
func RunImports(ctx context.Context, logger *slog.Logger, runner ImportRunner, schedule ImportSchedule) error {
	if schedule.Retry <= 0 || schedule.Idle < schedule.Retry {
		return errors.New("import worker schedule needs a positive retry no longer than its idle interval")
	}
	for ctx.Err() == nil {
		worked, err := runner.RunOne(ctx)
		if err != nil {
			logger.Warn("import worker poll failed", "code", "WORKER_POLL_FAILED")
		}
		if worked && err == nil {
			continue
		}
		wait := schedule.Retry
		if err == nil {
			wait = schedule.idleWait(ctx, logger, runner)
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-schedule.Wake:
			timer.Stop()
		case <-timer.C:
		}
	}
	return nil
}

func (s ImportSchedule) idleWait(ctx context.Context, logger *slog.Logger, runner ImportRunner) time.Duration {
	due, pending, err := runner.NextDue(ctx)
	if err != nil {
		logger.Warn("import worker poll failed", "code", "WORKER_POLL_FAILED")
		return s.Retry
	}
	if !pending {
		return s.Idle
	}
	return min(max(time.Until(due), s.Retry), s.Idle)
}

// WakeHandler answers POST /wake with 204 and signals the worker; signals
// coalesce, so any number of wakes costs at most one extra poll. It reveals
// nothing about the queue.
func WakeHandler(wake chan<- struct{}) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /wake", func(w http.ResponseWriter, _ *http.Request) {
		select {
		case wake <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

// ServeWake listens on address for WakeHandler until ctx ends. It returns once
// the listener is bound, so a bad address fails the worker at startup.
func ServeWake(ctx context.Context, logger *slog.Logger, address string, wake chan<- struct{}) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("import worker wake listener: %w", err)
	}
	server := &http.Server{Handler: WakeHandler(wake), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Warn("import worker wake listener stopped", "code", "WORKER_WAKE_LISTENER_FAILED")
		}
	}()
	return nil
}

// ObserveImportRun logs operational identities, outcome codes and timing without source text, object keys or signed URLs.
func ObserveImportRun(logger *slog.Logger) func(worker.Event) {
	return func(e worker.Event) {
		logger.Info("import worker event", "kind", e.Kind, "runId", e.RunID, "importId", e.ImportID, "stage", e.Stage, "attempt", e.Attempt, "code", e.Code, "durationMs", e.Duration.Milliseconds())
	}
}
