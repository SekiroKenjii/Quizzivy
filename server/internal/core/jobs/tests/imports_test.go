package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
)

type importRunner func(context.Context) (bool, error)

func (f importRunner) RunOne(ctx context.Context) (bool, error) { return f(ctx) }

func TestImportPollDrainsSeriallyWithoutWaitingBetweenCompletedRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	err := jobs.RunImports(ctx, slog.Default(), importRunner(func(context.Context) (bool, error) {
		calls++
		if calls == 3 {
			cancel()
		}
		return true, nil
	}), time.Hour)
	if err != nil || calls != 3 {
		t.Fatalf("serial drain: calls=%d err=%v", calls, err)
	}
}

func TestImportPollStopsPromptlyAndDoesNotLogRawFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var logs bytes.Buffer
	calls := 0
	err := jobs.RunImports(ctx, slog.New(slog.NewJSONHandler(&logs, nil)), importRunner(func(context.Context) (bool, error) {
		calls++
		cancel()
		return false, errors.New("private source content and storage credentials")
	}), time.Hour)
	if err != nil || calls != 1 || strings.Contains(logs.String(), "credentials") || !strings.Contains(logs.String(), "WORKER_POLL_FAILED") {
		t.Fatalf("unsafe polling: calls=%d err=%v logs=%s", calls, err, logs.String())
	}
}

func TestImportPollRejectsBusyLoopInterval(t *testing.T) {
	if err := jobs.RunImports(context.Background(), slog.Default(), importRunner(func(context.Context) (bool, error) {
		t.Fatal("invalid poll must not claim")
		return false, nil
	}), 0); err == nil {
		t.Fatal("accepted a zero poll interval")
	}
}
