package support

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"quizzivy/internal/modules/media/domain"
)

// Intake is one upload as it was received: its bytes in a temporary file,
// and what was read from them. Width and Height are set only for an image
// whose header gave its size. Close removes the file.
type Intake struct {
	Kind       domain.Kind
	MimeType   string
	Bytes      int64
	DurationMs *int
	Width      *int
	Height     *int
	Checksum   []byte
	file       *os.File
}

// Receive streams body to a temporary file and answers what it holds. It
// copies at most the audio limit and refuses anything longer with
// ErrTooLarge before the bytes are looked at; it then identifies the file by
// its content, refuses an image over the image limit with ErrImageTooLarge
// and audio over the duration limit with ErrTooLong, and reads an image's
// pixel size. The caller owns the Intake and must Close it.
func (s *Service) Receive(body io.Reader) (*Intake, error) {
	tmp, err := os.CreateTemp("", "quizzivy-upload-*")
	if err != nil {
		return nil, fmt.Errorf("media: temp file: %w", err)
	}
	in := &Intake{file: tmp}
	if err := s.measure(in, body); err != nil {
		in.Close()
		return nil, err
	}
	return in, nil
}

func (s *Service) measure(in *Intake, body io.Reader) error {
	hasher := sha256.New()
	size, err := BoundedCopy(io.MultiWriter(in.file, hasher), body, domain.MaxAudioBytes)
	if err != nil {
		return err
	}
	if size == 0 {
		return fmt.Errorf("%w: empty file", domain.ErrUnsupportedType)
	}
	kind, mime, durationMs, err := s.Identify(in.file, size)
	if err != nil {
		return err
	}
	if err := domain.Assets.CheckSize(kind, size); err != nil {
		return err
	}
	if err := domain.Assets.CheckDuration(durationMs); err != nil {
		return err
	}
	in.Kind, in.MimeType, in.Bytes, in.DurationMs, in.Checksum = kind, mime, size, durationMs, hasher.Sum(nil)
	if kind == domain.KindImage && s.Images != nil {
		if width, height, ok := s.Images.Image(in.file, size); ok {
			in.Width, in.Height = &width, &height
		}
	}
	return nil
}

// Body is the received bytes from their first, for the one who stores them.
func (in *Intake) Body() (io.Reader, error) {
	if _, err := in.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("media: rewind: %w", err)
	}
	return in.file, nil
}

// Close removes the temporary file. It is safe to call more than once.
func (in *Intake) Close() {
	_ = in.file.Close()
	_ = os.Remove(in.file.Name())
}
