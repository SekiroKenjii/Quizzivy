package classes

import (
	"context"
	"time"
)

type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) *Service { return &Service{store: store, now: time.Now} }

func (s *Service) Get(ctx context.Context, classID string) (Class, error) {
	return s.store.Get(ctx, classID)
}

func (s *Service) List(ctx context.Context) ([]Class, error) { return s.store.List(ctx) }

func (s *Service) Members(ctx context.Context, classID string) ([]Member, error) {
	return s.store.Members(ctx, classID)
}

func (s *Service) RemoveMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) error {
	if _, err := s.store.Get(ctx, classID); err != nil {
		return err
	}
	return s.store.RemoveMember(ctx, RemoveMemberInput{
		ClassID: classID, UserID: userID, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
