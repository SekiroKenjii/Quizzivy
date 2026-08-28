package media

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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
}

func NewService(store *Store, object ObjectStore) *Service {
	return &Service{store: store, object: object, now: time.Now}
}

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
	// A temp FILE, not a memory buffer. The probe needs io.ReaderAt, and 10 MB
	// per concurrent upload held in memory is the same shape of problem as
	// R-13 -- on a 512 MB machine it does not take many.
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

	// 2. Magic bytes, and 3. duration -- both from the bytes, never from the
	// filename or the Content-Type header.
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
	// §11.2's layout. The id is in the key, so a re-upload of identical bytes
	// gets a different key and never overwrites (§11.1) -- which is what lets a
	// frozen version reference an asset without copying the file.
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
		// Undo the object, or it is orphaned: nothing references it, nothing
		// lists it, and nothing will ever delete it.
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
	if len(name) > 200 {
		name = name[:200]
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

// SignedURLTTL is §11.2's ten minutes. Short because the URL IS the
// capability: one that outlives its purpose cannot be revoked afterwards.
const SignedURLTTL = 10 * time.Minute

// SignedURL mints a fresh URL for an asset, per request (§11.2).
func (s *Service) SignedURL(ctx context.Context, asset Asset) (string, error) {
	return s.object.SignedURL(ctx, asset.StorageKey, SignedURLTTL)
}
