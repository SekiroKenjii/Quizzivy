package adapters

import (
	"bytes"
	"context"
	"errors"
	"io"

	orgdomain "quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/platform/imagesafe"
	"quizzivy/internal/platform/storage"
)

// LogoStore is the object store behind the organization module's LogoStore
// port: a logo is stored once under the hash of its bytes and never replaced.
type LogoStore struct {
	Objects *storage.Client
}

// Put stores body under key.
func (s LogoStore) Put(ctx context.Context, key string, kind orgdomain.LogoType, body, digest []byte) error {
	return s.Objects.PutImmutable(ctx, key, string(kind), bytes.NewReader(body), int64(len(body)), digest)
}

// Get reads the bytes under key.
func (s LogoStore) Get(ctx context.Context, key string) ([]byte, error) {
	body, _, err := s.Objects.Open(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, orgdomain.ErrNoLogo
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	return io.ReadAll(io.LimitReader(body, orgdomain.MaxPNGBytes+1))
}

// Exists reports whether key holds an object.
func (s LogoStore) Exists(ctx context.Context, key string) (bool, error) {
	return s.Objects.Exists(ctx, key)
}

// LogoImages is the part of imagesafe.Processor that LogoChecker uses.
type LogoImages interface {
	Logo(ctx context.Context, body io.Reader, lim imagesafe.Limits) ([]byte, int, int, error)
}

var logoLimits = imagesafe.Limits{
	MaxFileBytes:    orgdomain.MaxPNGBytes,
	MinSide:         orgdomain.MinLogoSide,
	MaxSide:         orgdomain.MaxLogoSide,
	MaxDecodedBytes: imagesafe.MaxDecodedBytes,
	MaxScans:        imagesafe.MaxScans,
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// LogoChecker checks an upload for the organization module's LogoChecker
// port: a PNG of at most 1 MiB, square within 2% and 256 to 2048 pixels a
// side, decoded under the server's shared gate and encoded afresh; or an SVG
// of at most 256 KiB that holds only drawing, stored as it came.
type LogoChecker struct {
	Images LogoImages
}

// Check answers the domain's ErrLogo* and ErrSVGNotAllowed reasons for each
// way imagesafe refuses an upload and any other error as it is.
func (c LogoChecker) Check(ctx context.Context, body io.Reader) (orgdomain.LogoFile, error) {
	data, err := io.ReadAll(io.LimitReader(body, orgdomain.MaxPNGBytes+1))
	if err != nil {
		return orgdomain.LogoFile{}, err
	}
	if len(data) > orgdomain.MaxPNGBytes {
		return orgdomain.LogoFile{}, orgdomain.ErrLogoTooLarge
	}
	if bytes.HasPrefix(data, pngSignature) {
		png, w, h, err := c.Images.Logo(ctx, bytes.NewReader(data), logoLimits)
		if err != nil {
			return orgdomain.LogoFile{}, logoReason(err)
		}
		return orgdomain.LogoFile{Bytes: png, Type: orgdomain.LogoPNG, Width: w, Height: h}, nil
	}
	if !looksLikeSVG(data) {
		return orgdomain.LogoFile{}, orgdomain.ErrLogoUnsupported
	}
	svg, w, h, err := imagesafe.CheckSVG(bytes.NewReader(data), orgdomain.MaxSVGBytes)
	if err != nil {
		return orgdomain.LogoFile{}, logoReason(err)
	}
	return orgdomain.LogoFile{Bytes: svg, Type: orgdomain.LogoSVG, Width: w, Height: h}, nil
}

func looksLikeSVG(data []byte) bool {
	trimmed := bytes.TrimLeft(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte("<"))
}

func logoReason(err error) error {
	switch {
	case errors.Is(err, imagesafe.ErrTooLarge):
		return orgdomain.ErrLogoTooLarge
	case errors.Is(err, imagesafe.ErrUnsupported):
		return orgdomain.ErrLogoUnsupported
	case errors.Is(err, imagesafe.ErrUnreadable):
		return orgdomain.ErrLogoUnreadable
	case errors.Is(err, imagesafe.ErrDimensions):
		return orgdomain.ErrLogoDimensions
	case errors.Is(err, imagesafe.ErrSVGNotAllowed):
		return orgdomain.ErrSVGNotAllowed
	default:
		return err
	}
}
