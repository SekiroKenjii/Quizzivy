package support

import (
	"time"

	"quizzivy/internal/modules/notifications/domain"
)

// Service carries what the handlers share: the store and the clock.
type Service struct {
	Repo domain.Repository
	Now  func() time.Time
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo, Now: time.Now}
}
