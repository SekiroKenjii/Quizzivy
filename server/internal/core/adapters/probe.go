package adapters

import (
	"errors"
	"io"

	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/probe"
)

// AudioProbe adapts the platform probe to media's AudioProbe port.
type AudioProbe struct{}

func (AudioProbe) Audio(r io.ReaderAt, size int64) (string, int, error) {
	mime, durationMs, err := probe.Audio(r, size)
	switch {
	case errors.Is(err, probe.ErrUnsupportedType):
		return "", 0, errors.Join(mediadomain.ErrUnsupportedType, err)
	case errors.Is(err, probe.ErrUnmeasurable):
		return "", 0, errors.Join(mediadomain.ErrUnmeasurable, err)
	}
	return mime, durationMs, err
}
