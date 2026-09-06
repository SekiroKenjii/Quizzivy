package command

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/opt"
)

// Upload validates, stores the object, then records the row.
type Upload struct {
	Filename   string
	Body       io.Reader
	UploaderID string
	IP         string
	UserAgent  string
}

type UploadHandler struct {
	*support.Service
}

func (s UploadHandler) Handle(ctx context.Context, cmd Upload) (domain.Asset, error) {
	tmp, err := os.CreateTemp("", "quizzivy-upload-*")
	if err != nil {
		return domain.Asset{}, fmt.Errorf("media: temp file: %w", err)
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	hasher := sha256.New()
	size, err := support.BoundedCopy(io.MultiWriter(tmp, hasher), cmd.Body, domain.MaxBytes)
	if err != nil {
		return domain.Asset{}, err
	}
	if size == 0 {
		return domain.Asset{}, fmt.Errorf("%w: empty file", domain.ErrUnsupportedType)
	}
	kind, mime, durationMs, err := s.Identify(tmp, size)
	if err != nil {
		return domain.Asset{}, err
	}
	if err := domain.Assets.CheckDuration(durationMs); err != nil {
		return domain.Asset{}, err
	}

	checksum := hasher.Sum(nil)
	assetID, err := support.NewAssetID()
	if err != nil {
		return domain.Asset{}, err
	}
	key := fmt.Sprintf("%s/%s%s", kind, assetID, support.ExtensionFor(mime))

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return domain.Asset{}, fmt.Errorf("media: rewind: %w", err)
	}
	if err := s.Object.Put(ctx, key, mime, tmp, size); err != nil {
		return domain.Asset{}, err
	}

	asset, err := s.Repo.Insert(ctx, domain.InsertInput{
		ID:               assetID,
		Kind:             kind,
		StorageKey:       key,
		MimeType:         mime,
		Bytes:            size,
		DurationMs:       durationMs,
		OriginalFilename: support.SanitiseFilename(cmd.Filename),
		ChecksumSHA256:   checksum,
		UploaderID:       cmd.UploaderID,
		Now:              s.Now(),
		IP:               opt.String(cmd.IP),
		UserAgent:        opt.String(cmd.UserAgent),
	})
	if err != nil {
		_ = s.Object.Delete(ctx, key)
		return domain.Asset{}, err
	}
	return asset, nil
}
