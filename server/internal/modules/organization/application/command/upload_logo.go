package command

import (
	"context"
	"crypto/sha256"
	"io"

	"quizzivy/internal/modules/organization/application/internal/support"
	"quizzivy/internal/modules/organization/domain"
)

// UploadLogo checks Body as a logo and stores it under its content hash. It
// changes no setting: a save that names the hash does. It fails with
// domain.ErrStorageOff without object storage, and with the checker's reason
// for an upload that may not be a logo, having stored nothing.
type UploadLogo struct {
	Body io.Reader
}

// UploadedLogo is a stored logo: its hash, its type and its size in pixels.
type UploadedLogo struct {
	Logo   domain.Logo
	Width  int
	Height int
}

type UploadLogoHandler struct {
	*support.Service
}

func (h UploadLogoHandler) Handle(ctx context.Context, cmd UploadLogo) (UploadedLogo, error) {
	if h.Logos == nil || h.Checker == nil {
		return UploadedLogo{}, domain.ErrStorageOff
	}
	file, err := h.Checker.Check(ctx, cmd.Body)
	if err != nil {
		return UploadedLogo{}, err
	}
	digest := sha256.Sum256(file.Bytes)
	logo := domain.Logo{SHA256: digest[:], Type: file.Type}
	if err := h.Logos.Put(ctx, support.LogoKey(logo), file.Type, file.Bytes, logo.SHA256); err != nil {
		return UploadedLogo{}, err
	}
	return UploadedLogo{Logo: logo, Width: file.Width, Height: file.Height}, nil
}
