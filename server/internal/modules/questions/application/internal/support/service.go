package support

import (
	"context"
	"quizzivy/internal/modules/questions/application/ports"
	"quizzivy/internal/modules/questions/domain"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo  domain.Repository
	Media ports.MediaKinds
	Now   func() time.Time
}

func NewService(repo domain.Repository, media ports.MediaKinds) *Service {
	return &Service{Repo: repo, Media: media, Now: time.Now}
}

func (s *Service) ResolveMediaKind(ctx context.Context, assetID *string) (*string, error) {
	if assetID == nil {
		return nil, nil
	}
	if s.Media == nil {
		return nil, domain.ErrMediaNotFound
	}
	kind, err := s.Media.Kind(ctx, *assetID)
	if err != nil {
		return nil, err
	}
	return &kind, nil
}

func (s *Service) Write(ctx context.Context, req domain.WriteRequest, update bool) (domain.Question, error) {
	kind, err := s.ResolveMediaKind(ctx, req.Input.MediaAssetID)
	if err != nil {
		return domain.Question{}, err
	}
	if err := req.Input.Validate(kind); err != nil {
		return domain.Question{}, err
	}

	in := domain.WriteInput{
		ID:             req.ID,
		Input:          req.Input,
		MediaAssetKind: kind,
		ActorID:        req.ActorID,
		Now:            s.Now(),
		IP:             req.IP,
		UserAgent:      req.UserAgent,
	}
	if update {
		return s.Repo.Update(ctx, in)
	}
	return s.Repo.Create(ctx, in)
}
