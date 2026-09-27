package support

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"quizzivy/internal/modules/availability/domain"
)

// RefreshEvery is the longest the API serves a remembered answer to "is there
// a maintenance window?" before reading the database again.
const RefreshEvery = 30 * time.Second

const readTimeout = 2 * time.Second

// Snapshot remembers the next maintenance window in memory. It reads the
// database at most every RefreshEvery, only when asked, and one caller at a
// time, so an idle API never queries it and a busy one queries it twice a
// minute. Whether the window is under way is decided at each read, against the
// clock. A failed read keeps what was last read, or nothing, and is logged as
// MAINTENANCE_STATUS_UNAVAILABLE: the API stays open unless it knows otherwise.
type Snapshot struct {
	repo   domain.Repository
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	read   bool
	readAt time.Time
	window *domain.Window
}

// NewSnapshot builds a snapshot over the repository.
func NewSnapshot(repo domain.Repository, logger *slog.Logger) *Snapshot {
	return &Snapshot{repo: repo, logger: logger, now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Snapshot) SetClock(now func() time.Time) { s.now = now }

// Status returns the next window that has not ended and whether it is under way.
func (s *Snapshot) Status(ctx context.Context) domain.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if !s.read || now.Sub(s.readAt) >= RefreshEvery {
		s.refresh(ctx, now)
	}
	if s.window == nil || !now.Before(s.window.EndsAt) {
		return domain.Status{}
	}
	window := *s.window
	return domain.Status{Window: &window, Active: window.ActiveAt(now)}
}

func (s *Snapshot) refresh(ctx context.Context, now time.Time) {
	read, cancel := context.WithTimeout(context.WithoutCancel(ctx), readTimeout)
	defer cancel()
	window, err := s.repo.Next(read, now)
	s.read, s.readAt = true, now
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("maintenance status unavailable; serving the last state read",
				"code", "MAINTENANCE_STATUS_UNAVAILABLE", "err", err)
		}
		return
	}
	s.window = window
}
