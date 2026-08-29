package tests

import (
	"context"
	"time"
)

// Service applies the rules a schema cannot express, then writes.
type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) *Service {
	return &Service{store: store, now: time.Now}
}

type Request struct {
	ID        string
	ActorID   string
	IP        string
	UserAgent string
}

func (s *Service) List(ctx context.Context, in ListInput) ([]Test, string, error) {
	return s.store.List(ctx, in)
}

func (s *Service) Get(ctx context.Context, id string) (Test, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) ListVersions(ctx context.Context, testID string) ([]Version, error) {
	return s.store.ListVersions(ctx, testID)
}

func (s *Service) Preview(ctx context.Context, testID string, version int) (int, []PreviewQuestion, error) {
	return s.store.Preview(ctx, testID, version)
}

func (s *Service) Create(ctx context.Context, req Request, title string, description *string) (Test, error) {
	return s.store.Create(ctx, CreateInput{
		Title:       title,
		Description: description,
		ActorID:     req.ActorID,
		Now:         s.now(),
		IP:          req.IP,
		UserAgent:   req.UserAgent,
	})
}

func (s *Service) Update(ctx context.Context, req Request, in UpdateInput) (Test, error) {
	if err := in.Validate(); err != nil {
		return Test{}, err
	}
	return s.store.Update(ctx, UpdateRequest{
		ID:        req.ID,
		Input:     in,
		ActorID:   req.ActorID,
		Now:       s.now(),
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
}

func (s *Service) Duplicate(ctx context.Context, req Request) (Test, error) {
	return s.store.Duplicate(ctx, DuplicateInput{
		ID:        req.ID,
		ActorID:   req.ActorID,
		Now:       s.now(),
		IP:        req.IP,
		UserAgent: req.UserAgent,
	})
}
