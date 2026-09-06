package support

import (
	"context"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/stats"
	"time"
)

// Students carries what the students handlers share: their ports and the helpers they call.
type Students struct {
	Repo  domain.Students
	Stats stats.Source
	Now   func() time.Time
}

func NewStudents(repo domain.Students, stats stats.Source) *Students {
	return &Students{Repo: repo, Stats: stats, Now: time.Now}
}

func (s *Students) WithStats(ctx context.Context, student domain.Student) (domain.Student, error) {
	list := []domain.Student{student}
	if err := s.AttachStats(ctx, list); err != nil {
		return domain.Student{}, err
	}
	return list[0], nil
}

func (s *Students) AttachStats(ctx context.Context, students []domain.Student) error {
	ids := make([]string, len(students))
	for i, st := range students {
		ids[i] = st.ID
	}
	byStudent, err := s.Stats.StudentStats(ctx, ids)
	if err != nil {
		return err
	}
	for i := range students {
		students[i].Stats = byStudent[students[i].ID]
	}
	return nil
}
