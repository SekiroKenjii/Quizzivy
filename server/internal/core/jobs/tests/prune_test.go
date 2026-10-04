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
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/shared/cqrs"
)

func pruning(outcome func() (int64, error), budgets chan<- time.Duration) *notificationsapp.Application {
	return &notificationsapp.Application{Commands: notificationsapp.Commands{
		Prune: cqrs.HandlerFunc[notificationscommand.Prune, int64](func(ctx context.Context, _ notificationscommand.Prune) (int64, error) {
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

func TestTheNotificationPruneRunsOnceAtStartUnderItsOwnBudgetAndStopsWithTheProcess(t *testing.T) {
	for name, c := range map[string]struct {
		outcome func() (int64, error)
		logged  string
	}{
		"rows went":      {func() (int64, error) { return 3, nil }, `"msg":"pruned old notifications","rows":3`},
		"the run failed": {func() (int64, error) { return 0, errors.New("the database is away") }, `"msg":"notification prune failed","err":"the database is away"`},
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
				jobs.PruneNotifications(ctx, logger, pruning(c.outcome, budgets))
				close(done)
			}()

			select {
			case budget := <-budgets:
				if budget <= 0 || budget > time.Minute {
					t.Errorf("the first run had a budget of %s, want one of its own within a minute", budget)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the prune did not run at start")
			}
			select {
			case <-budgets:
				t.Fatal("the prune ran again within its day")
			case <-time.After(100 * time.Millisecond):
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("the prune kept running after the process was told to stop")
			}
			logMu.Lock()
			defer logMu.Unlock()
			if !strings.Contains(logs.String(), c.logged) {
				t.Errorf("the log is %s, want it to hold %s", logs.String(), c.logged)
			}
		})
	}
}
