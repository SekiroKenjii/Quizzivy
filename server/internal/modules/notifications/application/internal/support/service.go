package support

import (
	"time"

	"quizzivy/internal/modules/notifications/domain"
)

// Service carries what the handlers share: the store, the clock and the
// throttle on materialising due items.
type Service struct {
	Repo     domain.Repository
	Now      func() time.Time
	Throttle *Throttle
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo, Now: time.Now, Throttle: NewThrottle(domain.DueEvery)}
}
