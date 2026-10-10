// Package imagesafe turns an untrusted PNG or JPEG upload into a small square
// PNG without trusting anything the file says about itself. Each step that costs
// memory or time is preceded by a check that costs neither: the byte limit, the
// file signature, the header's dimensions and the memory the decode would need,
// estimated from the header. Only then is a slot taken from a Gate, so that at
// most a fixed number of images are decoded at once, and the pixels are decoded,
// cropped to the centred square, resampled with an exact box filter, turned
// upright by their EXIF orientation and encoded afresh. Nothing but pixels
// survives: the output carries no EXIF, ICC profile, text chunk or trailing
// bytes.
package imagesafe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"sync"
)

// MaxDecodedBytes is the most memory one decode may be estimated to need from the image's header; an image above it is refused before any pixel is allocated.
const MaxDecodedBytes = 48 << 20

// MaxScans is the most scans a JPEG may hold: the standard decoder makes a full pass over the image's coefficients for each scan and cannot be cancelled, so only the count bounds its time.
const MaxScans = 32

// Why an upload is refused. Each is checked before the next costs anything.
var (
	ErrTooLarge    = errors.New("imagesafe: file is over the size limit")
	ErrUnsupported = errors.New("imagesafe: not a png or a jpeg")
	ErrUnreadable  = errors.New("imagesafe: image cannot be read")
	ErrDimensions  = errors.New("imagesafe: sides are out of range, or the image needs too much memory or time to decode")
)

// Limits bounds what Square accepts and what it makes. MaxDecodedBytes and
// MaxScans are normally the package's constants of the same names; a zero
// MaxScans refuses every JPEG.
type Limits struct {
	MaxFileBytes    int64
	MinSide         int
	MaxSide         int
	OutSide         int
	MaxDecodedBytes int64
	MaxScans        int
}

// Processor decodes and normalises images, taking one slot of its Gate for the
// decode, the resample and the encode.
type Processor struct {
	gate *Gate
}

// New returns a Processor that decodes only while it holds a slot of gate.
// Share one gate between every Processor in the process, so that the bound is
// the server's.
func New(gate *Gate) *Processor {
	return &Processor{gate: gate}
}

// Square reads at most lim.MaxFileBytes from body and returns the centred
// square of the image as an lim.OutSide square PNG. It fails with ErrTooLarge,
// ErrUnsupported, ErrUnreadable or ErrDimensions, having decoded nothing for
// the first three and the fourth, and with the context's error if the context
// ends while it waits for a slot. A failure to read body is returned wrapped.
func (p *Processor) Square(ctx context.Context, body io.Reader, lim Limits) ([]byte, error) {
	data, err := readBounded(body, lim.MaxFileBytes)
	if err != nil {
		return nil, err
	}
	kind, err := sniff(data)
	if err != nil {
		return nil, err
	}
	head, err := inspect(data, kind, lim)
	if err != nil {
		return nil, err
	}
	release, err := p.gate.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	img, err := decode(data, kind, head)
	if err != nil {
		return nil, err
	}
	return encode(orient(boxSquare(img, lim.OutSide), head.orientation))
}

func readBounded(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("imagesafe: read: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}

const (
	kindPNG  = "png"
	kindJPEG = "jpeg"
)

var (
	pngSignature  = []byte("\x89PNG\r\n\x1a\n")
	jpegSignature = []byte{0xff, 0xd8, 0xff}
)

func sniff(data []byte) (string, error) {
	switch {
	case bytes.HasPrefix(data, pngSignature):
		return kindPNG, nil
	case bytes.HasPrefix(data, jpegSignature):
		return kindJPEG, nil
	default:
		return "", ErrUnsupported
	}
}

type header struct {
	width       int
	height      int
	orientation int
}

func inspect(data []byte, kind string, lim Limits) (header, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return header{}, fmt.Errorf("%w: %s", ErrUnreadable, err)
	}
	if format != kind {
		return header{}, ErrUnsupported
	}
	if cfg.Width < lim.MinSide || cfg.Height < lim.MinSide || cfg.Width > lim.MaxSide || cfg.Height > lim.MaxSide {
		return header{}, ErrDimensions
	}
	prof, err := profileOf(data, kind, cfg)
	if err != nil {
		return header{}, err
	}
	if prof.need > lim.MaxDecodedBytes || prof.scans > lim.MaxScans {
		return header{}, ErrDimensions
	}
	return header{width: cfg.Width, height: cfg.Height, orientation: prof.orientation}, nil
}

func decode(data []byte, kind string, head header) (img image.Image, err error) {
	defer func() {
		if recover() != nil {
			img, err = nil, ErrUnreadable
		}
	}()
	reader := bytes.NewReader(data)
	if kind == kindPNG {
		img, err = png.Decode(reader)
	} else {
		img, err = jpeg.Decode(reader)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, err)
	}
	if bounds := img.Bounds(); bounds.Dx() != head.width || bounds.Dy() != head.height {
		return nil, ErrUnreadable
	}
	return img, nil
}

func encode(img *image.NRGBA) ([]byte, error) {
	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("imagesafe: encode: %w", err)
	}
	return out.Bytes(), nil
}

// Gate lets a fixed number of callers hold it at once and makes the rest wait.
type Gate struct {
	slots chan struct{}
}

// NewGate returns a Gate of n slots, at least one.
func NewGate(n int) *Gate {
	return &Gate{slots: make(chan struct{}, max(n, 1))}
}

// Acquire waits for a slot and returns the function that gives it back, or the
// context's error if the context ends first. The release is safe to call more
// than once.
func (g *Gate) Acquire(ctx context.Context) (release func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case g.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-g.slots }) }, nil
}
