package classes

import (
	"context"
	"quizzivy/internal/paging"
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

func (s *Service) List(ctx context.Context, in ListInput) ([]Class, paging.Page, error) {
	return s.store.List(ctx, in)
}

func (s *Service) ListMine(ctx context.Context, userID string) ([]Class, error) {
	return s.store.ListMine(ctx, userID)
}

func (s *Service) Members(ctx context.Context, classID string, in MembersInput) ([]Member, paging.Page, error) {
	return s.store.Members(ctx, classID, in)
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

func (s *Service) AddMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) (Member, error) {
	return s.store.AddMember(ctx, AddMemberInput{
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

// Update edits a class's own fields.
func (s *Service) Update(ctx context.Context, classID string, in UpdateInput) (Class, error) {
	return s.store.Update(ctx, classID, in)
}

func (s *Service) Facets(ctx context.Context, query string) (Facets, error) {
	return s.store.Facets(ctx, query)
}

func (s *Service) Create(ctx context.Context, name string, description *string, selfJoin bool, actorID, ip, userAgent string) (Class, error) {
	return s.store.Create(ctx, CreateInput{
		Name: name, Description: description, SelfJoinEnabled: selfJoin, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
}

func (s *Service) Archive(ctx context.Context, classID string, archived bool, actorID, ip, userAgent string) (Class, error) {
	return s.store.Archive(ctx, ArchiveInput{
		ClassID: classID, Archived: archived, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
}
