package imagesafe

import (
	"context"
	"image/png"
	"io"
	"math"
)

const skewDivisor = 50

// Logo reads at most lim.MaxFileBytes from body and returns the PNG encoded
// afresh at the size it came in, with that size. It accepts a PNG only, whose
// sides lie between lim.MinSide and lim.MaxSide and differ by at most 2% of the
// longer one; it neither crops nor resamples, and the output carries no
// ancillary chunk and no trailing bytes. It fails with ErrTooLarge,
// ErrUnsupported (a JPEG is one), ErrUnreadable or ErrDimensions, having
// decoded nothing, and with the context's error if the context ends while it
// waits for a slot. A failure to read body is returned wrapped. lim.OutSide and
// lim.MaxScans are not used.
func (p *Processor) Logo(ctx context.Context, body io.Reader, lim Limits) (data []byte, width, height int, err error) {
	upload, err := readBounded(body, lim.MaxFileBytes)
	if err != nil {
		return nil, 0, 0, err
	}
	kind, err := sniff(upload)
	if err != nil {
		return nil, 0, 0, err
	}
	if kind != kindPNG {
		return nil, 0, 0, ErrUnsupported
	}
	head, err := inspect(upload, kind, lim)
	if err != nil {
		return nil, 0, 0, err
	}
	if !nearSquare(float64(head.width), float64(head.height)) {
		return nil, 0, 0, ErrDimensions
	}
	release, err := p.gate.Acquire(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	defer release()
	img, err := decode(upload, kind, head)
	if err != nil {
		return nil, 0, 0, err
	}
	data, err = encode(img, png.DefaultCompression)
	if err != nil {
		return nil, 0, 0, err
	}
	return data, head.width, head.height, nil
}

func nearSquare(width, height float64) bool {
	return skewDivisor*math.Abs(width-height) <= math.Max(width, height)
}
