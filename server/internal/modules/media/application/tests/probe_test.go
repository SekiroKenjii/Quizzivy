//go:build integration

package application_test

import (
	"errors"
	"io"

	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/probe"
)

type audioProbe struct{}

func (audioProbe) Audio(r io.ReaderAt, size int64) (string, int, error) {
	mime, durationMs, err := probe.Audio(r, size)
	switch {
	case errors.Is(err, probe.ErrUnsupportedType):
		return "", 0, errors.Join(domain.ErrUnsupportedType, err)
	case errors.Is(err, probe.ErrUnmeasurable):
		return "", 0, errors.Join(domain.ErrUnmeasurable, err)
	}
	return mime, durationMs, err
}
