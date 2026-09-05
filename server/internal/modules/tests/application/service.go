package application

import (
	"context"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/paging"
	"time"
)

// Service applies the rules a schema cannot express, then writes.
type Service struct {
	repo domain.Repository
	now  func() time.Time
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) List(ctx context.Context, in domain.ListInput) ([]domain.Test, paging.Page, error) {
	return s.repo.List(ctx, in)
}

func (s *Service) Tags(ctx context.Context, in domain.ListInput) ([]string, error) {
	return s.repo.Tags(ctx, in)
}

func (s *Service) Facets(ctx context.Context, in domain.ListInput) (domain.StatusFacets, error) {
	return s.repo.Facets(ctx, in)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Test, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) ListVersions(ctx context.Context, testID string) ([]domain.Version, error) {
	return s.repo.ListVersions(ctx, testID)
}

func (s *Service) Preview(ctx context.Context, testID string, version int) (int, []domain.PreviewQuestion, error) {
	return s.repo.Preview(ctx, testID, version)
}

func (s *Service) Create(ctx context.Context, req domain.Request, title string, description *string) (domain.Test, error) {
	return s.repo.Create(ctx, domain.CreateInput{
		Title:       title,
		Description: description,
		ActorID:     req.ActorID,
		Now:         s.now(),
		IP:          req.IP,
		UserAgent:   req.UserAgent,
	})
}

func (s *Service) Update(ctx context.Context, req domain.Request, in domain.UpdateInput) (domain.Test, error) {
	if err := in.Validate(); err != nil {
		return domain.Test{}, err
	}
	return s.repo.Update(ctx, domain.UpdateRequest{
		ID:        req.ID,
		Input:     in,
		ActorID:   req.ActorID,
		Now:       s.now(),
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
}

func (s *Service) Duplicate(ctx context.Context, req domain.Request) (domain.Test, error) {
	return s.repo.Duplicate(ctx, domain.DuplicateInput{
		ID:        req.ID,
		ActorID:   req.ActorID,
		Now:       s.now(),
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
}
