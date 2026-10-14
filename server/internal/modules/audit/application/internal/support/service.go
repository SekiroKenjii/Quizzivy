package support

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/modules/audit/application/ports"
	"quizzivy/internal/modules/audit/domain"
)

// Service carries what the handlers share: the log, the clock and the ports
// that name a viewer's zone and sign an actor's photo.
type Service struct {
	Repo    domain.Repository
	Now     func() time.Time
	Zones   ports.Zones
	Avatars ports.Avatars
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo, Now: time.Now}
}

// LocationOf is the zone viewerID's days are counted in: the one they stored
// or the centre's default.
func (s *Service) LocationOf(ctx context.Context, viewerID string) (string, *time.Location, error) {
	zone := domain.DefaultZone
	if s.Zones != nil && viewerID != "" {
		stored, err := s.Zones.ZoneOf(ctx, viewerID)
		if err != nil {
			return "", nil, fmt.Errorf("audit: viewer's zone: %w", err)
		}
		if stored != "" {
			zone = stored
		}
	}
	return zone, domain.LocationOf(zone), nil
}

// Present reads records for a screen or a file, resolving the labels of the
// rows they are about in one batch per kind of row and signing each actor's
// photo once.
func (s *Service) Present(ctx context.Context, records []domain.Record) ([]domain.Entry, error) {
	labels, err := s.Repo.Labels(ctx, refsOf(records))
	if err != nil {
		return nil, err
	}
	signed := map[string]string{}
	entries := make([]domain.Entry, len(records))
	for i, rec := range records {
		var label *string
		if ref, ok := rec.Ref(); ok {
			if name, found := labels[ref]; found {
				label = &name
			}
		}
		entries[i] = domain.NewEntry(rec, label, s.avatarOf(ctx, rec.Actor, signed))
	}
	return entries, nil
}

func (s *Service) avatarOf(ctx context.Context, actor *domain.Actor, signed map[string]string) string {
	if actor == nil || actor.AvatarKey == "" || s.Avatars == nil {
		return ""
	}
	url, seen := signed[actor.AvatarKey]
	if !seen {
		if got, err := s.Avatars.AvatarURL(ctx, actor.AvatarKey); err == nil {
			url = got
		}
		signed[actor.AvatarKey] = url
	}
	return url
}

func refsOf(records []domain.Record) []domain.EntityRef {
	seen := make(map[domain.EntityRef]bool, len(records))
	var refs []domain.EntityRef
	for _, rec := range records {
		if ref, ok := rec.Ref(); ok && !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	return refs
}
