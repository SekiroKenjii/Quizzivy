package adapters

import (
	"context"
	"errors"
	"io"
	"log/slog"

	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/imagesafe"
)

var photoLimits = imagesafe.Limits{
	MaxFileBytes:    2 << 20,
	MinSide:         200,
	MaxSide:         2048,
	OutSide:         256,
	MaxDecodedBytes: imagesafe.MaxDecodedBytes,
	MaxScans:        imagesafe.MaxScans,
}

// ImageProcessor is the part of imagesafe.Processor that Photos uses.
type ImageProcessor interface {
	Square(ctx context.Context, body io.Reader, lim imagesafe.Limits) ([]byte, error)
}

// Photos makes a profile photo from an upload for the identity module's
// PhotoProcessor port: a PNG or a JPEG of at most 2 MiB, each side 200 to 2048
// pixels and at most 32 scans, stored as a 256 x 256 PNG. The Processor's gate
// is the server's bound on images decoded at once. A decoder panic that the
// processor survived is logged at Warn, since the upload only sees it as an
// unreadable image.
type Photos struct {
	Processor ImageProcessor
	Log       *slog.Logger
}

// Square answers the identity domain's ErrAvatar* reason for each way
// imagesafe refuses an upload and any other error as it is.
func (p Photos) Square(ctx context.Context, body io.Reader) ([]byte, error) {
	photo, err := p.Processor.Square(ctx, body, photoLimits)
	switch {
	case err == nil:
		return photo, nil
	case errors.Is(err, imagesafe.ErrTooLarge):
		return nil, identitydomain.ErrAvatarTooLarge
	case errors.Is(err, imagesafe.ErrUnsupported):
		return nil, identitydomain.ErrAvatarUnsupported
	case errors.Is(err, imagesafe.ErrUnreadable):
		p.logDecoderPanic(ctx, err)
		return nil, identitydomain.ErrAvatarUnreadable
	case errors.Is(err, imagesafe.ErrDimensions):
		return nil, identitydomain.ErrAvatarDimensions
	default:
		return nil, err
	}
}

func (p Photos) logDecoderPanic(ctx context.Context, err error) {
	if p.Log != nil && errors.Is(err, imagesafe.ErrDecoderPanic) {
		p.Log.WarnContext(ctx, "profile photo decoder panicked", "err", err)
	}
}
