package adapters

import (
	"context"
	"errors"
	"io"

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

// Photos makes a profile photo from an upload for the identity module's
// PhotoProcessor port: a PNG or a JPEG of at most 2 MiB, each side 200 to 2048
// pixels and at most 32 scans, stored as a 256 x 256 PNG. The Processor's gate is the server's bound
// on images decoded at once.
type Photos struct{ Processor *imagesafe.Processor }

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
		return nil, identitydomain.ErrAvatarUnreadable
	case errors.Is(err, imagesafe.ErrDimensions):
		return nil, identitydomain.ErrAvatarDimensions
	default:
		return nil, err
	}
}
