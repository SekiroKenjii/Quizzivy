package support

import (
	"context"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/stats"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo        domain.Repository
	Stats       stats.Source
	ClassScores stats.ClassSource
	Avatars     ports.Avatars
	Now         func() time.Time
}

func NewService(repo domain.Repository, stats stats.Source) *Service {
	return &Service{Repo: repo, Stats: stats, Now: time.Now}
}

func (s *Service) AttachStats(ctx context.Context, scope access.Scope, members []domain.Member) error {
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	byStudent, err := s.Stats.StudentStats(ctx, scope, ids)
	if err != nil {
		return err
	}
	for i := range members {
		members[i].Stats = byStudent[members[i].UserID]
	}
	return nil
}

// AttachAverage sets AverageScore on each class from one read of the classes'
// scores. Without a score source every class keeps none. The classes were
// reached by the caller already, so no scope is applied here.
func (s *Service) AttachAverage(ctx context.Context, classes ...*domain.Class) error {
	if s.ClassScores == nil || len(classes) == 0 {
		return nil
	}
	ids := make([]string, len(classes))
	for i, c := range classes {
		ids[i] = c.ID
	}
	scores, err := s.ClassScores.ClassScores(ctx, ids)
	if err != nil {
		return err
	}
	for _, c := range classes {
		if score, ok := scores[c.ID]; ok {
			c.AverageScore = &score
		}
	}
	return nil
}
