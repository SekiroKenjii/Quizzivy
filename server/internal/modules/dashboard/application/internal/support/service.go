package support

import (
	"quizzivy/internal/modules/dashboard/application/ports"
	"quizzivy/internal/modules/dashboard/domain"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo          domain.Repository
	Now           func() time.Time
	Zones         ports.Zones
	Notifications ports.UnreadNotifications
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo, Now: time.Now}
}
