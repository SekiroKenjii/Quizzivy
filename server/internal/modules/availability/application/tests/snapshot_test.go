package application_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/modules/availability/application"
	"quizzivy/internal/modules/availability/application/query"
	"quizzivy/internal/modules/availability/domain"
)

type fakeRepo struct {
	reads  atomic.Int32
	window *domain.Window
	err    error
	gate   chan struct{}
}

func (f *fakeRepo) Next(ctx context.Context, _ time.Time) (*domain.Window, error) {
	f.reads.Add(1)
	if f.gate != nil {
		<-f.gate
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return f.window, f.err
}

type clock struct{ now time.Time }

func (c *clock) read() time.Time          { return c.now }
func (c *clock) advance(by time.Duration) { c.now = c.now.Add(by) }

var t0 = time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)

func build(repo *fakeRepo, logs *bytes.Buffer) (*application.Application, *clock) {
	var logger *slog.Logger
	if logs != nil {
		logger = slog.New(slog.NewTextHandler(logs, nil))
	}
	app := application.New(repo, logger)
	c := &clock{now: t0}
	app.SetClock(c.read)
	return app, c
}

func status(t *testing.T, app *application.Application) domain.Status {
	t.Helper()
	got, err := app.Queries.CurrentWindow.Handle(context.Background(), query.CurrentWindow{})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTheDatabaseIsReadAtMostEveryThirtySeconds(t *testing.T) {
	repo := &fakeRepo{}
	app, c := build(repo, nil)

	status(t, app)
	c.advance(29 * time.Second)
	status(t, app)
	if got := repo.reads.Load(); got != 1 {
		t.Fatalf("%d reads within thirty seconds, want 1", got)
	}
	c.advance(time.Second)
	status(t, app)
	if got := repo.reads.Load(); got != 2 {
		t.Errorf("%d reads after thirty seconds, want 2", got)
	}
}

func TestAWindowBecomesActiveWithoutAnotherRead(t *testing.T) {
	repo := &fakeRepo{window: &domain.Window{StartsAt: t0.Add(10 * time.Second), EndsAt: t0.Add(time.Hour)}}
	app, c := build(repo, nil)

	before := status(t, app)
	if before.Window == nil || before.Active {
		t.Fatalf("before the start: %+v, want the window, not yet active", before)
	}
	c.advance(15 * time.Second)
	during := status(t, app)
	if !during.Active {
		t.Errorf("fifteen seconds in: %+v, want active", during)
	}
	if got := repo.reads.Load(); got != 1 {
		t.Errorf("%d reads, want the one", got)
	}
}

func TestAnEndedWindowIsNoLongerReported(t *testing.T) {
	repo := &fakeRepo{window: &domain.Window{StartsAt: t0.Add(-time.Minute), EndsAt: t0.Add(10 * time.Second)}}
	app, c := build(repo, nil)

	if !status(t, app).Active {
		t.Fatal("the window is not active while it runs")
	}
	c.advance(20 * time.Second)
	if got := status(t, app); got.Window != nil || got.Active {
		t.Errorf("after the end: %+v, want nothing", got)
	}
}

func TestAFailedFirstReadLeavesTheAPIOpenAndSaysSo(t *testing.T) {
	var logs bytes.Buffer
	repo := &fakeRepo{err: errors.New("connection refused")}
	app, _ := build(repo, &logs)

	if got := status(t, app); got.Window != nil || got.Active {
		t.Errorf("with no window ever read: %+v, want open", got)
	}
	if !strings.Contains(logs.String(), "MAINTENANCE_STATUS_UNAVAILABLE") {
		t.Errorf("the failure was not logged with its code: %q", logs.String())
	}
}

func TestAFailedReadKeepsTheWindowItLastRead(t *testing.T) {
	repo := &fakeRepo{window: &domain.Window{StartsAt: t0.Add(-time.Minute), EndsAt: t0.Add(time.Hour)}}
	app, c := build(repo, &bytes.Buffer{})

	status(t, app)
	repo.window, repo.err = nil, errors.New("the database went down for the maintenance")
	c.advance(time.Minute)
	if got := status(t, app); !got.Active {
		t.Errorf("after a failed refresh during the window: %+v, want it still active", got)
	}
	if got := repo.reads.Load(); got != 2 {
		t.Errorf("%d reads, want the refresh to have been tried", got)
	}
}

func TestConcurrentCallersShareOneRead(t *testing.T) {
	repo := &fakeRepo{gate: make(chan struct{})}
	app, _ := build(repo, nil)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = app.Queries.CurrentWindow.Handle(context.Background(), query.CurrentWindow{})
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(repo.gate)
	wg.Wait()
	if got := repo.reads.Load(); got != 1 {
		t.Errorf("%d reads for twenty concurrent callers, want 1", got)
	}
}

func TestACancelledRequestDoesNotSpoilTheRead(t *testing.T) {
	repo := &fakeRepo{window: &domain.Window{StartsAt: t0.Add(-time.Minute), EndsAt: t0.Add(time.Hour)}}
	app, _ := build(repo, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := app.Queries.CurrentWindow.Handle(ctx, query.CurrentWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Active {
		t.Errorf("a request cancelled by its client read %+v, want the active window", got)
	}
}
