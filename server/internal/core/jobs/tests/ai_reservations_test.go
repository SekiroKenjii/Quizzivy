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
	aiapp "quizzivy/internal/modules/ai/application"
	aicommand "quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/cqrs"
)

func sweeping(outcome func() (domain.Released, error), budgets chan<- time.Duration) *aiapp.Application {
	return &aiapp.Application{Commands: aiapp.Commands{
		ReleaseStale: cqrs.HandlerFunc[aicommand.ReleaseStale, domain.Released](func(ctx context.Context, _ aicommand.ReleaseStale) (domain.Released, error) {
			deadline, bounded := ctx.Deadline()
			if !bounded {
				budgets <- 0
			} else {
				budgets <- time.Until(deadline)
			}
			return outcome()
		}),
	}}
}

func TestTheAIReservationSweepRunsOnceAtStartUnderItsOwnBudgetAndStopsWithTheProcess(t *testing.T) {
	for name, c := range map[string]struct {
		outcome func() (domain.Released, error)
		logged  string
	}{
		"credits came back": {func() (domain.Released, error) { return domain.Released{Reservations: 2, Credits: 300}, nil }, `"msg":"released stale ai reservations","reservations":2,"credits":300`},
		"nothing was stale": {func() (domain.Released, error) { return domain.Released{}, nil }, ``},
		"the run failed":    {func() (domain.Released, error) { return domain.Released{}, errors.New("the database is away") }, `"msg":"ai reservation sweep failed","err":"the database is away"`},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var logs bytes.Buffer
			var logMu sync.Mutex
			logger := slog.New(slog.NewJSONHandler(writerFunc(func(p []byte) (int, error) {
				logMu.Lock()
				defer logMu.Unlock()
				return logs.Write(p)
			}), nil))
			budgets := make(chan time.Duration, 8)
			done := make(chan struct{})
			go func() {
				jobs.ReleaseStaleAiReservations(ctx, logger, sweeping(c.outcome, budgets))
				close(done)
			}()

			select {
			case budget := <-budgets:
				if budget <= 0 || budget > time.Minute {
					t.Errorf("the first run had a budget of %s, want one of its own within a minute", budget)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the sweep did not run at start")
			}
			select {
			case <-budgets:
				t.Fatal("the sweep ran again within its hour")
			case <-time.After(100 * time.Millisecond):
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("the sweep kept running after the process was told to stop")
			}
			logMu.Lock()
			defer logMu.Unlock()
			if !strings.Contains(logs.String(), c.logged) {
				t.Errorf("the log is %s, want it to hold %s", logs.String(), c.logged)
			}
			if c.logged == "" && logs.Len() != 0 {
				t.Errorf("a sweep that found nothing logged %s", logs.String())
			}
		})
	}
}
