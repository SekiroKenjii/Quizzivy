package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
	"quizzivy/internal/modules/imports/application/worker"
)

type sweeps struct {
	mu    sync.Mutex
	calls []time.Time
	fail  int
}

func (s *sweeps) sweep(ctx context.Context, _ time.Time) (worker.Swept, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, time.Now())
	if _, ok := ctx.Deadline(); !ok {
		return worker.Swept{}, errors.New("a sweep ran without a time budget")
	}
	if len(s.calls) <= s.fail {
		return worker.Swept{Closed: 1}, errors.New("originals/secret-import/secret-source unreachable")
	}
	return worker.Swept{Removed: 2}, nil
}

func (s *sweeps) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func TestTheRetentionSweepRunsAtStartAndRetriesSoonAfterAFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer
	var logMu sync.Mutex
	logger := slog.New(slog.NewJSONHandler(writerFunc(func(p []byte) (int, error) {
		logMu.Lock()
		defer logMu.Unlock()
		return logs.Write(p)
	}), nil))
	s := &sweeps{fail: 1}
	done := make(chan struct{})
	go func() {
		jobs.SweepImportFiles(ctx, logger, s.sweep, jobs.ImportRetention{Every: time.Hour, Retry: 30 * time.Millisecond, Budget: time.Second})
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for s.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if s.count() != 2 {
		t.Fatalf("a failed sweep was retried %d times", s.count()-1)
	}
	time.Sleep(100 * time.Millisecond)
	if s.count() != 2 {
		t.Fatalf("a successful sweep ran again within its period: %d", s.count())
	}
	cancel()
	<-done
	logMu.Lock()
	defer logMu.Unlock()
	if !strings.Contains(logs.String(), "IMPORT_RETENTION_FAILED") || strings.Contains(logs.String(), "secret") {
		t.Fatalf("logs: %s", logs.String())
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
