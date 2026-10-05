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
	Images ports.ImageProbe
	Now    func() time.Time
	TTL    time.Duration
	Quota  int64
}

func NewService(repo domain.Repository, object ports.ObjectStore, probe ports.AudioProbe) *Service {
	return &Service{Repo: repo, Object: object, Probe: probe, Now: time.Now, TTL: DefaultSignedURLTTL, Quota: domain.DefaultOwnerQuotaBytes}
}

// WithImageProbe sets what reads an image's pixel size at upload. Without
// one, images are stored with no size.
func (s *Service) WithImageProbe(images ports.ImageProbe) *Service {
	s.Images = images
	return s
}

// WithOwnerQuota sets the bytes one owner's library may hold, from
// configuration. A non-positive value keeps the default rather than refusing
// every upload.
func (s *Service) WithOwnerQuota(bytes int64) *Service {
	if bytes > 0 {
		s.Quota = bytes
	}
	return s
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

// Describe fills what a library row shows beyond its own columns: the
// published versions that reference it, how many live questions use it, and
// a signed URL, since the bucket is private (§11.2).
func (s *Service) Describe(ctx context.Context, assets []domain.Asset) error {
	ids := make([]string, len(assets))
	for i := range assets {
		ids[i] = assets[i].ID
	}
	refs, err := s.Repo.ReferencesFor(ctx, ids)
	if err != nil {
		return err
	}
	questions, err := s.Repo.QuestionCounts(ctx, ids)
	if err != nil {
		return err
	}
	for i := range assets {
		url, err := s.Object.SignedURL(ctx, assets[i].StorageKey, s.TTL)
		if err != nil {
			return err
		}
		assets[i].URL = url
		assets[i].UsedIn = refs[assets[i].ID]
		assets[i].UsageCount = len(assets[i].UsedIn)
		assets[i].QuestionCount = questions[assets[i].ID]
	}
	return nil
}

func (s *Service) Mint(ctx context.Context, asset domain.Asset) (model.SignedURLResult, error) {
	url, err := s.Object.SignedURL(ctx, asset.StorageKey, s.TTL)
	if err != nil {
		return model.SignedURLResult{}, err
	}
	return model.SignedURLResult{URL: url, ExpiresAt: s.Now().Add(s.TTL)}, nil
}
