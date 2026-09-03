package media

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"quizzivy/internal/paging"
	"strings"
	"time"

	"quizzivy/internal/media/probe"
)

// ObjectStore is the slice of internal/storage this package uses.
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Delete(ctx context.Context, key string) error
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type Service struct {
	store  *Store
	object ObjectStore
	now    func() time.Time
	ttl    time.Duration
}

func NewService(store *Store, object ObjectStore) *Service {
	return &Service{store: store, object: object, now: time.Now, ttl: DefaultSignedURLTTL}
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
//
// The ORDER is the contract (§11.1): size, then magic bytes, then duration. A
// 50 MB upload is cut off by the first check, before anything parses it -- so a
// file that is too big never becomes a parsing problem.
//
// The row is written AFTER the object lands, and the object is deleted if the
// row fails. Either half alone is worse than neither: a row without an object
// is a library entry that 404s when a student presses play, and an object
// without a row is a file nothing will ever reference or clean up.
func (s *Service) Upload(ctx context.Context, in UploadInput) (Asset, error) {
	tmp, err := os.CreateTemp("", "quizzivy-upload-*")
	if err != nil {
		return Asset{}, fmt.Errorf("media: temp file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	// 1. Size.
	hasher := sha256.New()
	size, err := boundedCopy(io.MultiWriter(tmp, hasher), in.Body, MaxBytes)
	if err != nil {
		return Asset{}, err
	}
	if size == 0 {
		return Asset{}, fmt.Errorf("%w: empty file", probe.ErrUnsupportedType)
	}
	kind, mime, durationMs, err := identify(tmp, size)
	if err != nil {
		return Asset{}, err
	}
	if durationMs != nil && *durationMs > MaxDurationMs {
		return Asset{}, fmt.Errorf("%w: %d ms", ErrTooLong, *durationMs)
	}

	checksum := hasher.Sum(nil)
	assetID, err := newAssetID()
	if err != nil {
		return Asset{}, err
	}
	key := fmt.Sprintf("%s/%s%s", kind, assetID, extensionFor(mime))

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return Asset{}, fmt.Errorf("media: rewind: %w", err)
	}
	if err := s.object.Put(ctx, key, mime, tmp, size); err != nil {
		return Asset{}, err
	}

	asset, err := s.store.Insert(ctx, InsertInput{
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
		return Asset{}, err
	}
	return asset, nil
}

// identify sniffs the container and, for audio, probes the duration.
func identify(r io.ReaderAt, size int64) (Kind, string, *int, error) {
	head := make([]byte, 16)
	if n, err := r.ReadAt(head, 0); err != nil && n < 12 {
		return "", "", nil, fmt.Errorf("%w: too short to identify", probe.ErrUnsupportedType)
	}

	if mime := sniffImage(head); mime != "" {
		return KindImage, mime, nil, nil
	}

	mime, durationMs, err := probe.Audio(r, size)
	if err != nil {
		return "", "", nil, err
	}
	return KindAudio, mime, &durationMs, nil
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

// sanitiseFilename keeps a recognisable label without keeping a path.
//
// The name is shown in the library and nothing else -- it never becomes a
// storage key, which is derived from the asset id. So the only job here is to
// stop a display string carrying directory separators or arriving empty.
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

var errNoID = errors.New("media: could not generate an asset id")

// DefaultSignedURLTTL is §11.2's ten minutes. Short because the URL IS the
// capability: one that outlives its purpose cannot be revoked afterwards.
//
// The value a Service actually signs with is its ttl field, set from
// SIGNED_URL_TTL. This is the fallback for a Service built without one.
const DefaultSignedURLTTL = 10 * time.Minute

// SignedURL mints a fresh URL for an asset, per request (§11.2).
func (s *Service) SignedURL(ctx context.Context, asset Asset) (string, error) {
	return s.object.SignedURL(ctx, asset.StorageKey, s.ttl)
}

// ErrForbidden is a student asking for an asset they cannot reach. Deliberately
// indistinguishable from an asset that does not exist: telling the caller which
// one it was turns the endpoint into an oracle for which asset ids are real.
var ErrForbidden = errors.New("media: asset not reachable by this student")

// SignedURLResult is a minted capability and the moment it stops working.
type SignedURLResult struct {
	URL       string
	ExpiresAt time.Time
}

// MintForStudent issues a signed URL only for an asset the student can reach
// through an attempt of their own (§11.2).
//
// Authorization is decided before the URL is minted, and the same error is
// returned whether the asset is unreachable or absent.
func (s *Service) MintForStudent(ctx context.Context, studentID, assetID string) (SignedURLResult, error) {
	ok, err := ReachableByStudent(ctx, s.store.pool, studentID, assetID)
	if err != nil {
		return SignedURLResult{}, err
	}
	if !ok {
		return SignedURLResult{}, ErrForbidden
	}

	asset, err := s.store.Get(ctx, assetID)
	if errors.Is(err, ErrNotFound) {
		return SignedURLResult{}, ErrForbidden
	}
	if err != nil {
		return SignedURLResult{}, err
	}
	return s.mint(ctx, asset)
}

// mint is the one place a signature and its stated expiry are produced, so the
// expiresAt a client caches against cannot drift from the TTL actually signed.
func (s *Service) mint(ctx context.Context, asset Asset) (SignedURLResult, error) {
	url, err := s.object.SignedURL(ctx, asset.StorageKey, s.ttl)
	if err != nil {
		return SignedURLResult{}, err
	}
	return SignedURLResult{URL: url, ExpiresAt: s.now().Add(s.ttl)}, nil
}

// List returns a page of the library with a signed URL on every item, since the
// bucket is private and a listing without URLs cannot render a preview (§11.2).
func (s *Service) List(ctx context.Context, in ListInput) ([]Asset, paging.Page, error) {
	assets, page, err := s.store.List(ctx, in)
	if err != nil {
		return nil, paging.Page{}, err
	}
	for i := range assets {
		url, err := s.object.SignedURL(ctx, assets[i].StorageKey, s.ttl)
		if err != nil {
			return nil, paging.Page{}, err
		}
		assets[i].URL = url
		refs, err := CountReferences(ctx, s.store.pool, assets[i].ID)
		if err != nil {
			return nil, paging.Page{}, err
		}
		assets[i].UsageCount = refs
	}
	return assets, page, nil
}

// Get resolves one live asset, so another package can render an attachment
// without reaching into media's store.
func (s *Service) Get(ctx context.Context, id string) (Asset, error) {
	return s.store.Get(ctx, id)
}

// Delete soft-deletes an unreferenced asset.
func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	if in.Now.IsZero() {
		in.Now = s.now()
	}
	return s.store.SoftDelete(ctx, in)
}
