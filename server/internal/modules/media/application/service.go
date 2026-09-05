package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/paging"
	"strings"
	"time"
)

// ObjectStore is the slice of internal/storage this package uses.
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// AudioProbe measures an upload that is not an image: its MIME type and its
// duration, failing with the domain's ErrUnsupportedType or ErrUnmeasurable.
type AudioProbe interface {
	Audio(r io.ReaderAt, size int64) (mime string, durationMs int, err error)
}

type Service struct {
	repo   domain.Repository
	object ObjectStore
	probe  AudioProbe
	now    func() time.Time
	ttl    time.Duration
}

func NewService(repo domain.Repository, object ObjectStore, probe AudioProbe) *Service {
	return &Service{repo: repo, object: object, probe: probe, now: time.Now, ttl: DefaultSignedURLTTL}
}

// WithSignedURLTTL sets the signature lifetime from configuration. A
// non-positive value keeps the default rather than minting URLs that are
// already expired.
func (s *Service) WithSignedURLTTL(ttl time.Duration) *Service {
	if ttl > 0 {
		s.ttl = ttl
	}
	return s
}

// SignedURLTTL is the lifetime this service signs with. Exported because the
// Cache-Control directive on a signed-URL response has to be derived from the
// same value -- a cache entry outliving its signature is what §11.2's max-age
// exists to prevent, and two independent copies of "ten minutes" is how that
// stops being true.
func (s *Service) SignedURLTTL() time.Duration { return s.ttl }

type UploadInput struct {
	Filename   string
	Body       io.Reader
	UploaderID string
	IP         string
	UserAgent  string
}

// Upload validates, stores the object, then records the row.
func (s *Service) Upload(ctx context.Context, in UploadInput) (domain.Asset, error) {
	tmp, err := os.CreateTemp("", "quizzivy-upload-*")
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: temp file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	hasher := sha256.New()
	size, err := boundedCopy(io.MultiWriter(tmp, hasher), in.Body, domain.MaxBytes)
	if err != nil {
		return domain.Asset{}, err
	}
	if size == 0 {
		return domain.Asset{}, fmt.Errorf("%w: empty file", domain.ErrUnsupportedType)
	}
	kind, mime, durationMs, err := s.identify(tmp, size)
	if err != nil {
		return domain.Asset{}, err
	}
	if err := domain.Assets.CheckDuration(durationMs); err != nil {
		return domain.Asset{}, err
	}

	checksum := hasher.Sum(nil)
	assetID, err := newAssetID()
	if err != nil {
		return domain.Asset{}, err
	}
	key := fmt.Sprintf("%s/%s%s", kind, assetID, extensionFor(mime))

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return domain.Asset{}, fmt.Errorf("media: rewind: %w", err)
	}
	if err := s.object.Put(ctx, key, mime, tmp, size); err != nil {
		return domain.Asset{}, err
	}

	asset, err := s.repo.Insert(ctx, domain.InsertInput{
		ID:               assetID,
		Kind:             kind,
		StorageKey:       key,
		MimeType:         mime,
		Bytes:            size,
		DurationMs:       durationMs,
		OriginalFilename: sanitiseFilename(in.Filename),
		ChecksumSHA256:   checksum,
		UploaderID:       in.UploaderID,
		Now:              s.now(),
		IP:               optional(in.IP),
		UserAgent:        optional(in.UserAgent),
	})
	if err != nil {
		_ = s.object.Delete(ctx, key)
		return domain.Asset{}, err
	}
	return asset, nil
}

func (s *Service) identify(r io.ReaderAt, size int64) (domain.Kind, string, *int, error) {
	head := make([]byte, 16)
	if n, err := r.ReadAt(head, 0); err != nil && n < 12 {
		return "", "", nil, fmt.Errorf("%w: too short to identify", domain.ErrUnsupportedType)
	}

	if mime := sniffImage(head); mime != "" {
		return domain.KindImage, mime, nil, nil
	}

	mime, durationMs, err := s.probe.Audio(r, size)
	if err != nil {
		return "", "", nil, err
	}
	return domain.KindAudio, mime, &durationMs, nil
}

func extensionFor(mime string) string {
	switch mime {
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4", "audio/aac":
		return ".m4a"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	}
	return ""
}

func sanitiseFilename(name string) string {
	name = path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "tệp-không-tên"
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	return name
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// DefaultSignedURLTTL is §11.2's ten minutes. Short because the URL IS the
// capability: one that outlives its purpose cannot be revoked afterwards.
const DefaultSignedURLTTL = 10 * time.Minute

// SignedURL mints a fresh URL for an asset, per request (§11.2).
func (s *Service) SignedURL(ctx context.Context, asset domain.Asset) (string, error) {
	return s.object.SignedURL(ctx, asset.StorageKey, s.ttl)
}

// SignedURLResult is a minted capability and the moment it stops working.
type SignedURLResult struct {
	URL       string
	ExpiresAt time.Time
}

// MintForStudent issues a signed URL only for an asset the student can reach
// through an attempt of their own (§11.2).
func (s *Service) MintForStudent(ctx context.Context, studentID, assetID string) (SignedURLResult, error) {
	ok, err := s.repo.ReachableByStudent(ctx, studentID, assetID)
	if err != nil {
		return SignedURLResult{}, err
	}
	if !ok {
		return SignedURLResult{}, domain.ErrForbidden
	}

	asset, err := s.repo.Get(ctx, assetID)
	if errors.Is(err, domain.ErrNotFound) {
		return SignedURLResult{}, domain.ErrForbidden
	}
	if err != nil {
		return SignedURLResult{}, err
	}
	return s.mint(ctx, asset)
}

func (s *Service) mint(ctx context.Context, asset domain.Asset) (SignedURLResult, error) {
	url, err := s.object.SignedURL(ctx, asset.StorageKey, s.ttl)
	if err != nil {
		return SignedURLResult{}, err
	}
	return SignedURLResult{URL: url, ExpiresAt: s.now().Add(s.ttl)}, nil
}

// List returns a page of the library with a signed URL on every item, since the
// bucket is private and a listing without URLs cannot render a preview (§11.2).
func (s *Service) List(ctx context.Context, in domain.ListInput) ([]domain.Asset, paging.Page, error) {
	assets, page, err := s.repo.List(ctx, in)
	if err != nil {
		return nil, paging.Page{}, err
	}
	ids := make([]string, len(assets))
	for i := range assets {
		ids[i] = assets[i].ID
	}
	refs, err := s.repo.ReferencesFor(ctx, ids)
	if err != nil {
		return nil, paging.Page{}, err
	}
	for i := range assets {
		url, err := s.object.SignedURL(ctx, assets[i].StorageKey, s.ttl)
		if err != nil {
			return nil, paging.Page{}, err
		}
		assets[i].URL = url
		assets[i].UsedIn = refs[assets[i].ID]
		assets[i].UsageCount = len(assets[i].UsedIn)
	}
	return assets, page, nil
}

func (s *Service) TotalBytes(ctx context.Context, kind *domain.Kind) (int64, error) {
	return s.repo.TotalBytes(ctx, kind)
}

// Get resolves one live asset, so another package can render an attachment
// without reaching into media's store.
func (s *Service) Get(ctx context.Context, id string) (domain.Asset, error) {
	return s.repo.Get(ctx, id)
}

// Delete soft-deletes an unreferenced asset.
func (s *Service) Delete(ctx context.Context, in domain.DeleteInput) error {
	if in.Now.IsZero() {
		in.Now = s.now()
	}
	return s.repo.SoftDelete(ctx, in)
}
