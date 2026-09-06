package support

import (
	"quizzivy/internal/modules/tests/domain"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo domain.Repository
	Now  func() time.Time
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo, Now: time.Now}
}
