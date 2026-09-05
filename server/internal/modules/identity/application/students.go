package application

import (
	"context"
	"time"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
	"quizzivy/internal/shared/stats"
)

// Students is the teacher's view of student accounts: the roster with its
// figures, and the writes that create, edit and re-issue a password.
type Students struct {
	repo  domain.Students
	stats stats.Source
	now   func() time.Time
}

func NewStudents(repo domain.Students, stats stats.Source) *Students {
	return &Students{repo: repo, stats: stats, now: time.Now}
}

func (s *Students) List(ctx context.Context, q domain.StudentQuery) ([]domain.Student, paging.Page, error) {
	found, page, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, paging.Page{}, err
	}
	if err := s.attachStats(ctx, found); err != nil {
		return nil, paging.Page{}, err
	}
	return found, page, nil
}

func (s *Students) Facets(ctx context.Context, q domain.StudentQuery) (domain.StudentFacets, error) {
	return s.repo.Facets(ctx, q)
}

func (s *Students) Get(ctx context.Context, id string) (domain.Student, error) {
	student, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Student{}, err
	}
	return s.withStats(ctx, student)
}

// Create adds a student who signs in with the temporary password it returns.
func (s *Students) Create(ctx context.Context, req domain.WriteRequest, in domain.NewStudent) (domain.Student, string, error) {
	temporary, hash, err := temporaryPassword(ctx)
	if err != nil {
		return domain.Student{}, "", err
	}
	in.Hash = hash
	in.Now = s.now()
	student, err := s.repo.Create(ctx, req, in)
	if err != nil {
		return domain.Student{}, "", err
	}
	student, err = s.withStats(ctx, student)
	return student, temporary, err
}

func (s *Students) Update(ctx context.Context, req domain.WriteRequest, in domain.StudentPatch) (domain.Student, error) {
	in.Now = s.now()
	student, err := s.repo.Update(ctx, req, in)
	if err != nil {
		return domain.Student{}, err
	}
	return s.withStats(ctx, student)
}

// ResetPassword issues a fresh temporary password and ends every session the student has.
func (s *Students) ResetPassword(ctx context.Context, req domain.WriteRequest, id string) (string, error) {
	temporary, hash, err := temporaryPassword(ctx)
	if err != nil {
		return "", err
	}
	if err := s.repo.ResetPassword(ctx, req, id, hash, s.now()); err != nil {
		return "", err
	}
	return temporary, nil
}

func (s *Students) withStats(ctx context.Context, student domain.Student) (domain.Student, error) {
	list := []domain.Student{student}
	if err := s.attachStats(ctx, list); err != nil {
		return domain.Student{}, err
	}
	return list[0], nil
}

func (s *Students) attachStats(ctx context.Context, students []domain.Student) error {
	ids := make([]string, len(students))
	for i, st := range students {
		ids[i] = st.ID
	}
	byStudent, err := s.stats.StudentStats(ctx, ids)
	if err != nil {
		return err
	}
	for i := range students {
		students[i].Stats = byStudent[students[i].ID]
	}
	return nil
}
