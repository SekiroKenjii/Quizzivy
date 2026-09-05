package application

import (
	"context"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/paging"
	"quizzivy/internal/shared/stats"
	"time"
)

type Service struct {
	repo  domain.Repository
	stats stats.Source
	now   func() time.Time
}

func NewService(repo domain.Repository, stats stats.Source) *Service {
	return &Service{repo: repo, stats: stats, now: time.Now}
}

func (s *Service) Get(ctx context.Context, classID string) (domain.Class, error) {
	return s.repo.Get(ctx, classID)
}

func (s *Service) List(ctx context.Context, in domain.ListInput) ([]domain.Class, paging.Page, error) {
	return s.repo.List(ctx, in)
}

func (s *Service) ListMine(ctx context.Context, userID string) ([]domain.MyClass, error) {
	return s.repo.ListMine(ctx, userID)
}

func (s *Service) Members(ctx context.Context, classID string, in domain.MembersInput) ([]domain.Member, paging.Page, error) {
	members, page, err := s.repo.Members(ctx, classID, in)
	if err != nil {
		return nil, paging.Page{}, err
	}
	if err := s.attachStats(ctx, members); err != nil {
		return nil, paging.Page{}, err
	}
	return members, page, nil
}

func (s *Service) attachStats(ctx context.Context, members []domain.Member) error {
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	byStudent, err := s.stats.StudentStats(ctx, ids)
	if err != nil {
		return err
	}
	for i := range members {
		members[i].Stats = byStudent[members[i].UserID]
	}
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) error {
	if _, err := s.repo.Get(ctx, classID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, domain.RemoveMemberInput{
		ClassID: classID, UserID: userID, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
}

func (s *Service) AddMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) (domain.Member, error) {
	member, err := s.repo.AddMember(ctx, domain.AddMemberInput{
		ClassID: classID, UserID: userID, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
	if err != nil {
		return domain.Member{}, err
	}
	members := []domain.Member{member}
	if err := s.attachStats(ctx, members); err != nil {
		return domain.Member{}, err
	}
	return members[0], nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// Update edits a class's own fields.
func (s *Service) Update(ctx context.Context, classID string, in domain.UpdateInput) (domain.Class, error) {
	return s.repo.Update(ctx, classID, in)
}

func (s *Service) Facets(ctx context.Context, query string) (domain.Facets, error) {
	return s.repo.Facets(ctx, query)
}

func (s *Service) Create(ctx context.Context, name string, description *string, selfJoin bool, actorID, ip, userAgent string) (domain.Class, error) {
	return s.repo.Create(ctx, domain.CreateInput{
		Name: name, Description: description, SelfJoinEnabled: selfJoin, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
}

func (s *Service) Archive(ctx context.Context, classID string, archived bool, actorID, ip, userAgent string) (domain.Class, error) {
	return s.repo.Archive(ctx, domain.ArchiveInput{
		ClassID: classID, Archived: archived, ActorUserID: actorID,
		Now: s.now(), IP: optional(ip), UserAgent: optional(userAgent),
	})
}
