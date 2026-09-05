package application

import (
	"context"

	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/paging"
)

type Service struct {
	repo domain.Repository
}

func New(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Summary(ctx context.Context) (domain.Summary, error) {
	return s.repo.Summary(ctx)
}

func (s *Service) List(ctx context.Context, q domain.ListQuery) ([]domain.Recent, paging.Page, error) {
	return s.repo.List(ctx, q)
}
