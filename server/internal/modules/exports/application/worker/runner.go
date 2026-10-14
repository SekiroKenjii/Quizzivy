// Package worker builds the data exports: it claims a queued export, streams
// the centre's tables into one ZIP of CSV files on the worker's disk, stores
// the ZIP in the private bucket and reports the export ready, or failed.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"quizzivy/internal/modules/exports/application/ports"
	"quizzivy/internal/modules/exports/domain"
)

const settleBudget = 10 * time.Second

// Runner executes the exports one at a time. A supervisor decides when it
// runs: RunOne claims at most one export and returns, and NextDue says when
// the next one will be due. A nil Logger logs nothing, a nil Now is the
// clock, and a Timeout of zero is domain.Timeout.
type Runner struct {
	Queue   domain.Queue
	Source  domain.Source
	Store   ports.Store
	Table   ports.NewTable
	WorkDir string
	Timeout time.Duration
	Logger  *slog.Logger
	Now     func() time.Time
}

// NextDue reports when the queue will next have an export for this runner;
// false means none is waiting.
func (r Runner) NextDue(ctx context.Context) (time.Time, bool, error) {
	return r.Queue.NextDue(ctx)
}

// RunOne claims the oldest waiting export and builds it. It reports whether it
// found one. A build that fails marks the export failed and is not an error of
// RunOne; an error is a failure to claim or to record the outcome.
func (r Runner) RunOne(ctx context.Context) (bool, error) {
	r = r.withDefaults()
	claimed, err := r.Queue.Claim(ctx)
	if errors.Is(err, domain.ErrNoWork) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	started := time.Now()
	size, buildErr := r.build(ctx, claimed)
	outcome, err := r.settle(ctx, claimed, size, buildErr)
	r.Logger.Info("data export", "exportId", claimed.ID, "attempt", claimed.Attempts, "outcome", outcome,
		"bytes", size, "durationMs", time.Since(started).Milliseconds())
	return true, err
}

func (r Runner) withDefaults() Runner {
	if r.Logger == nil {
		r.Logger = slog.New(slog.DiscardHandler)
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	if r.Timeout <= 0 {
		r.Timeout = domain.Timeout
	}
	return r
}

func (r Runner) build(ctx context.Context, claimed domain.Export) (int64, error) {
	work, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	path := filepath.Join(r.WorkDir, "export-"+claimed.ID+".zip")
	defer func() { _ = os.Remove(path) }()
	size, err := archiver{source: r.Source, table: r.Table, now: r.Now}.write(work, path, claimed.IncludeAnswers)
	if err != nil {
		return 0, err
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("exports: open the archive: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := r.Store.Put(work, domain.StorageKey(claimed.ID), domain.ArchiveContentType, file, size); err != nil {
		return 0, storageFailure{err}
	}
	return size, nil
}

type storageFailure struct{ error }

func (r Runner) settle(ctx context.Context, claimed domain.Export, size int64, buildErr error) (string, error) {
	write, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleBudget)
	defer cancel()
	switch {
	case buildErr == nil:
		err := r.Queue.Complete(write, domain.Completion{
			ID: claimed.ID, Attempt: claimed.Attempts, StorageKey: domain.StorageKey(claimed.ID), Bytes: size,
		})
		return "ready", lost(err)
	case ctx.Err() != nil:
		return "released", lost(r.Queue.Release(write, claimed.ID, claimed.Attempts))
	default:
		code := codeOf(buildErr)
		return code, lost(r.Queue.Fail(write, claimed.ID, claimed.Attempts, code))
	}
}

func lost(err error) error {
	if errors.Is(err, domain.ErrLeaseLost) {
		return nil
	}
	return err
}

func codeOf(err error) string {
	var storage storageFailure
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return domain.CodeTimeout
	case errors.As(err, &storage):
		return domain.CodeStorage
	}
	return domain.CodeFailed
}
