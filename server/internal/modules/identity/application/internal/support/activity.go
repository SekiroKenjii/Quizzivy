package support

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"quizzivy/internal/modules/identity/domain"
)

const (
	// ActivityThrottle is how often one instance lets one user through: the first request of a local day always passes.
	ActivityThrottle = 5 * time.Minute
	// ActivityFlushEvery is how often waiting entries are written.
	ActivityFlushEvery = 30 * time.Second
	// ActivityFlushAt is how many waiting entries are written at once without waiting for the timer.
	ActivityFlushAt     = 200
	activityFlushBudget = 10 * time.Second
)

type stamped struct {
	at  time.Time
	day time.Time
}

// Activity collects the authenticated requests of users and writes them in batches, so a busy API costs one small transaction every few seconds and an idle one writes nothing.
type Activity struct {
	Store domain.Activity
	Now   func() time.Time
	Zone  *time.Location
	Log   *slog.Logger

	mu      sync.Mutex
	seen    map[string]stamped
	pending map[domain.ActivityEntry]struct{}
	full    chan struct{}
}

// NewActivity returns an Activity writing to store, counting days in zone.
func NewActivity(store domain.Activity, zone *time.Location) *Activity {
	return &Activity{
		Store:   store,
		Now:     time.Now,
		Zone:    zone,
		seen:    map[string]stamped{},
		pending: map[domain.ActivityEntry]struct{}{},
		full:    make(chan struct{}, 1),
	}
}

// Stamp notes that userID made an authenticated request now. The same user is let through at most once every ActivityThrottle, and again at the first request of a new local day.
func (a *Activity) Stamp(userID string) {
	now := a.Now()
	local := now.In(a.Zone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	a.mu.Lock()
	defer a.mu.Unlock()
	if last, ok := a.seen[userID]; ok && last.day.Equal(day) && now.Sub(last.at) < ActivityThrottle {
		return
	}
	a.seen[userID] = stamped{at: now, day: day}
	a.pending[domain.ActivityEntry{UserID: userID, At: now, Day: day}] = struct{}{}
	if len(a.pending) >= ActivityFlushAt {
		select {
		case a.full <- struct{}{}:
		default:
		}
	}
}

// Flush writes the waiting entries, and forgets throttle records old enough to let their user through again. A failed write is logged and its entries dropped: activity is a statistic, and a database that is down is not helped by a queue that grows.
func (a *Activity) Flush(ctx context.Context) {
	a.mu.Lock()
	if len(a.pending) == 0 {
		a.mu.Unlock()
		return
	}
	entries := make([]domain.ActivityEntry, 0, len(a.pending))
	for entry := range a.pending {
		entries = append(entries, entry)
	}
	clear(a.pending)
	cutoff := a.Now().Add(-ActivityThrottle)
	for id, last := range a.seen {
		if last.at.Before(cutoff) {
			delete(a.seen, id)
		}
	}
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), activityFlushBudget)
	defer cancel()
	if err := a.Store.RecordActivity(ctx, entries); err != nil {
		a.logger().Warn("activity not recorded", "entries", len(entries), "err", err)
	}
}

// Run flushes every ActivityFlushEvery and as soon as ActivityFlushAt entries wait, and once more when ctx ends.
func (a *Activity) Run(ctx context.Context) {
	ticker := time.NewTicker(ActivityFlushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.Flush(ctx)
			return
		case <-ticker.C:
			a.Flush(ctx)
		case <-a.full:
			a.Flush(ctx)
		}
	}
}

func (a *Activity) logger() *slog.Logger {
	if a.Log == nil {
		return slog.Default()
	}
	return a.Log
}
