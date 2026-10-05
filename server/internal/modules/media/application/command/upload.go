package command

import (
	"context"
	"fmt"
	"io"
	"quizzivy/internal/modules/media/application/internal/support"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
)

// Upload validates, stores the object, then records the row in the
// uploader's library. DefaultMaxPlays, when set, is the play limit a new
// question starts from; it is refused for anything but audio before the
// object is stored.
type Upload struct {
	Filename        string
	Body            io.Reader
	UploaderID      string
	DefaultMaxPlays *int
	IP              string
	UserAgent       string
}

type UploadHandler struct {
	*support.Service
}

func (s UploadHandler) Handle(ctx context.Context, cmd Upload) (domain.Asset, error) {
	intake, err := s.Receive(cmd.Body)
	if err != nil {
		return domain.Asset{}, err
	}
	defer intake.Close()

	if err := domain.Assets.CheckPlayLimit(intake.Kind, cmd.DefaultMaxPlays); err != nil {
		return domain.Asset{}, err
	}
	held, err := s.Repo.Usage(ctx, access.Scope{UserID: cmd.UploaderID})
	if err != nil {
		return domain.Asset{}, err
	}
	if err := domain.Assets.CheckQuota(held.AudioBytes+held.ImageBytes, intake.Bytes, s.Quota); err != nil {
		return domain.Asset{}, err
	}

	assetID, err := support.NewAssetID()
	if err != nil {
		return domain.Asset{}, err
	}
	key := fmt.Sprintf("%s/%s%s", intake.Kind, assetID, support.ExtensionFor(intake.MimeType))

	body, err := intake.Body()
	if err != nil {
		return domain.Asset{}, err
	}
	if err := s.Object.Put(ctx, key, intake.MimeType, body, intake.Bytes); err != nil {
		return domain.Asset{}, err
	}

	asset, err := s.Repo.Insert(ctx, domain.InsertInput{
		ID:               assetID,
		Kind:             intake.Kind,
		StorageKey:       key,
		MimeType:         intake.MimeType,
		Bytes:            intake.Bytes,
		DurationMs:       intake.DurationMs,
		OriginalFilename: support.SanitiseFilename(cmd.Filename),
		ChecksumSHA256:   intake.Checksum,
		UploaderID:       cmd.UploaderID,
		DefaultMaxPlays:  cmd.DefaultMaxPlays,
		Width:            intake.Width,
		Height:           intake.Height,
		QuotaBytes:       s.Quota,
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
