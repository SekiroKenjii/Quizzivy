package application

import (
	"context"
	"time"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/paging"
)

// Service is the teacher's and the student's use cases over assignments.
type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, in domain.ListInput) ([]domain.Assignment, paging.Page, error) {
	return s.repo.List(ctx, in)
}

func (s *Service) Facets(ctx context.Context, in domain.ListInput) (domain.Facets, error) {
	return s.repo.Facets(ctx, in)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Assignment, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, req domain.Request, in domain.WriteInput) (domain.Assignment, error) {
	return s.repo.Create(ctx, req, in)
}

func (s *Service) Update(ctx context.Context, req domain.Request, in domain.WriteInput) (domain.Assignment, error) {
	return s.repo.Update(ctx, req, in)
}

func (s *Service) Reopen(ctx context.Context, req domain.Request, closesAt time.Time, reason string, now time.Time) (domain.Assignment, error) {
	return s.repo.Reopen(ctx, req, closesAt, reason, now)
}

func (s *Service) ForStudent(ctx context.Context, studentID string, now time.Time) (domain.StudentSections, error) {
	return s.repo.ForStudent(ctx, studentID, now)
}

func (s *Service) StudentDetail(ctx context.Context, id, studentID string) (domain.StudentDetail, error) {
	return s.repo.StudentDetail(ctx, id, studentID)
}
