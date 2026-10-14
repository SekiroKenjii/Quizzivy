package support

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"quizzivy/internal/modules/organization/domain"
)

// RefreshEvery is the longest the server answers from a remembered document
// before reading the database again, and so the longest a save on another
// server takes to apply here.
const RefreshEvery = 30 * time.Second

const readTimeout = 2 * time.Second

// Snapshot remembers the settings document in memory. It reads the database
// at most every RefreshEvery, only when asked, and one caller at a time, so an
// idle server never queries it. A save on this server replaces it at once. A
// failed read keeps what was last read and is logged as
// ORGANIZATION_SETTINGS_UNAVAILABLE; with nothing read yet it is returned.
type Snapshot struct {
	repo   domain.Repository
	logger *slog.Logger
	now    func() time.Time

	mu       sync.Mutex
	read     bool
	readAt   time.Time
	settings domain.Settings
}

// NewSnapshot builds a snapshot over the repository.
func NewSnapshot(repo domain.Repository, logger *slog.Logger) *Snapshot {
	return &Snapshot{repo: repo, logger: logger, now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Snapshot) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Settings returns the remembered document, reading it first when it is
// older than RefreshEvery.
func (s *Snapshot) Settings(ctx context.Context) (domain.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.read && now.Sub(s.readAt) < RefreshEvery {
		return s.settings, nil
	}
	read, cancel := context.WithTimeout(context.WithoutCancel(ctx), readTimeout)
	defer cancel()
	settings, err := s.repo.Read(read)
	if err != nil {
		if !s.read {
			return domain.Settings{}, err
		}
		s.readAt = now
		if s.logger != nil {
			s.logger.Warn("organization settings unavailable; serving the last document read",
				"code", "ORGANIZATION_SETTINGS_UNAVAILABLE", "err", err)
		}
		return s.settings, nil
	}
	s.read, s.readAt, s.settings = true, now, settings
	return settings, nil
}

// Remember replaces the document with one this server just saved.
func (s *Snapshot) Remember(settings domain.Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.read, s.readAt, s.settings = true, s.now(), settings
}
