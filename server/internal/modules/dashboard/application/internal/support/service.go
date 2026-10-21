package support

import (
	"quizzivy/internal/modules/dashboard/application/ports"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/limits"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo          domain.Repository
	Now           func() time.Time
	Zones         ports.Zones
	Notifications ports.UnreadNotifications
	Branding      ports.Branding
	Recent        ports.RecentChanges
	Credits       ports.AICredits
	LimitEnv      limits.Env
	Location      *time.Location
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo, Now: time.Now, Location: time.FixedZone(domain.DefaultZone, 7*60*60)}
}
