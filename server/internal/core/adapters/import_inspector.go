package adapters

import (
	"context"
	"errors"
	"io"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/word"
	"time"
)

// ImportInspector performs bounded native package validation; extraction and semantic findings belong to a durable processing run.
type ImportInspector struct{}

func (ImportInspector) Inspect(ctx context.Context, r io.ReaderAt, size int64) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := word.Inspect(ctx, r, size, word.DefaultLimits())
	switch {
	case err == nil:
		return nil
	case errors.Is(err, word.ErrLimit):
		return domain.ErrTooLarge
	case errors.Is(err, word.ErrActiveContent), errors.Is(err, word.ErrLegacyOrLocked):
		return domain.ErrUnsupported
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return domain.ErrBusy
	default:
		return domain.ErrInvalid
	}
}
