package support

import (
	"context"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/stats"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo  domain.Repository
	Stats stats.Source
	Now   func() time.Time
}

func NewService(repo domain.Repository, stats stats.Source) *Service {
	return &Service{Repo: repo, Stats: stats, Now: time.Now}
}

func (s *Service) AttachStats(ctx context.Context, members []domain.Member) error {
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	byStudent, err := s.Stats.StudentStats(ctx, ids)
	if err != nil {
		return err
	}
	for i := range members {
		members[i].Stats = byStudent[members[i].UserID]
	}
	return nil
}
