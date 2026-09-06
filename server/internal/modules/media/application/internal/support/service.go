package support

import (
	"context"
	"fmt"
	"io"
	"quizzivy/internal/modules/media/application/model"
	"quizzivy/internal/modules/media/application/ports"
	"quizzivy/internal/modules/media/domain"
	"time"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo   domain.Repository
	Object ports.ObjectStore
	Probe  ports.AudioProbe
	Now    func() time.Time
	TTL    time.Duration
}

func NewService(repo domain.Repository, object ports.ObjectStore, probe ports.AudioProbe) *Service {
	return &Service{Repo: repo, Object: object, Probe: probe, Now: time.Now, TTL: DefaultSignedURLTTL}
}

// WithSignedURLTTL sets the signature lifetime from configuration. A
// non-positive value keeps the default rather than minting URLs that are
// already expired.
func (s *Service) WithSignedURLTTL(ttl time.Duration) *Service {
	if ttl > 0 {
		s.TTL = ttl
	}
	return s
}

// SignedURLTTL is the lifetime this service signs with. Exported because the
// Cache-Control directive on a signed-URL response has to be derived from the
// same value -- a cache entry outliving its signature is what §11.2's max-age
// exists to prevent, and two independent copies of "ten minutes" is how that
// stops being true.
func (s *Service) SignedURLTTL() time.Duration { return s.TTL }

func (s *Service) Identify(r io.ReaderAt, size int64) (domain.Kind, string, *int, error) {
	head := make([]byte, 16)
	if n, err := r.ReadAt(head, 0); err != nil && n < 12 {
		return "", "", nil, fmt.Errorf("%w: too short to identify", domain.ErrUnsupportedType)
	}

	if mime := SniffImage(head); mime != "" {
		return domain.KindImage, mime, nil, nil
	}

	mime, durationMs, err := s.Probe.Audio(r, size)
	if err != nil {
		return "", "", nil, err
	}
	return domain.KindAudio, mime, &durationMs, nil
}

func (s *Service) Mint(ctx context.Context, asset domain.Asset) (model.SignedURLResult, error) {
	url, err := s.Object.SignedURL(ctx, asset.StorageKey, s.TTL)
	if err != nil {
		return model.SignedURLResult{}, err
	}
	return model.SignedURLResult{URL: url, ExpiresAt: s.Now().Add(s.TTL)}, nil
}
