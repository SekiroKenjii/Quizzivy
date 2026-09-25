package word

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
)

// ErrUnsupportedImage identifies embedded content that cannot be safely normalized without changing its semantics.
var ErrUnsupportedImage = errors.New("word: unsupported image")

// ImageLimits bounds compressed input, decoded pixels and the normalized output of one image.
type ImageLimits struct {
	Bytes       int64
	Pixels      int64
	Dimension   int
	OutputBytes int64
}

// DefaultImageLimits returns conservative development limits; processing remains sequential inside an isolated worker.
func DefaultImageLimits() ImageLimits {
	return ImageLimits{Bytes: 10 << 20, Pixels: 16_000_000, Dimension: 8192, OutputBytes: 20 << 20}
}

// NormalizedImage is a decoded static raster re-encoded as PNG without source metadata; it remains private until explicitly bound to learner content.
type NormalizedImage struct {
	Part          string
	Width, Height int
	PNG           []byte
}

// ReadImage validates and normalizes one embedded PNG/JPEG without fetching relationships, executing objects or exposing original bytes as media.
func ReadImage(ctx context.Context, src io.ReaderAt, size int64, part string, limits Limits, images ImageLimits) (NormalizedImage, error) {
	if images.Bytes < 1 || images.Bytes > 25<<20 || images.Pixels < 1 || images.Pixels > 16_000_000 || images.Dimension < 1 || images.Dimension > 8192 || images.OutputBytes < 1 || images.OutputBytes > 32<<20 {
		return NormalizedImage{}, fmt.Errorf("%w: image configuration", ErrLimit)
	}
	a, err := openArchive(ctx, src, size, limits)
	if err != nil {
		return NormalizedImage{}, err
	}
	data, err := imageBytes(ctx, a, part, images.Bytes)
	if err != nil {
		return NormalizedImage{}, err
	}
	return normalizeImage(ctx, part, data, images)
}

func imageBytes(ctx context.Context, a *archive, part string, maxBytes int64) ([]byte, error) {
	file, found := a.files[part]
	if !found {
		return nil, fmt.Errorf("%w: image part absent", ErrInvalidPackage)
	}
	if file.UncompressedSize64 > uint64(maxBytes) {
		return nil, fmt.Errorf("%w: image bytes", ErrLimit)
	}
	r, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: image part", ErrInvalidPackage)
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: r}, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: image read: %w", ErrInvalidPackage, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: image bytes", ErrLimit)
	}
	return data, nil
}

func normalizeImage(ctx context.Context, part string, data []byte, limits ImageLimits) (NormalizedImage, error) {
	config, format, err := image.DecodeConfig(contextReader{ctx: ctx, reader: bytes.NewReader(data)})
	if err != nil || (format != "png" && format != "jpeg") {
		return NormalizedImage{}, ErrUnsupportedImage
	}
	if config.Width < 1 || config.Height < 1 || config.Width > limits.Dimension || config.Height > limits.Dimension || int64(config.Width)*int64(config.Height) > limits.Pixels {
		return NormalizedImage{}, fmt.Errorf("%w: image dimensions", ErrLimit)
	}
	if format == "png" && unsupportedPNGMetadata(data) || format == "jpeg" && hasJPEGExif(data) {
		return NormalizedImage{}, ErrUnsupportedImage
	}
	decoded, _, err := image.Decode(contextReader{ctx: ctx, reader: bytes.NewReader(data)})
	if err != nil {
		return NormalizedImage{}, fmt.Errorf("%w: image decode", ErrInvalidPackage)
	}
	if decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return NormalizedImage{}, fmt.Errorf("%w: image dimensions changed", ErrInvalidPackage)
	}
	out := &boundedImageWriter{ctx: ctx, remaining: limits.OutputBytes}
	if err := png.Encode(out, decoded); err != nil {
		return NormalizedImage{}, err
	}
	return NormalizedImage{Part: part, Width: config.Width, Height: config.Height, PNG: out.Bytes()}, ctx.Err()
}

func unsupportedPNGMetadata(data []byte) bool {
	for at := 8; len(data)-at >= 12; {
		size := uint64(binary.BigEndian.Uint32(data[at : at+4]))
		if kind := string(data[at+4 : at+8]); kind == "acTL" || kind == "eXIf" {
			return true
		}
		if size > uint64(len(data)-at-12) {
			return false
		}
		at += int(size) + 12
	}
	return false
}

func hasJPEGExif(data []byte) bool {
	for at := 2; at+4 <= len(data); {
		if data[at] != 0xff {
			return false
		}
		marker := data[at+1]
		if marker == 0xda || marker == 0xd9 {
			return false
		}
		if marker == 0xff {
			at++
			continue
		}
		size := int(binary.BigEndian.Uint16(data[at+2 : at+4]))
		if size < 2 || size > len(data)-at-2 {
			return false
		}
		if marker == 0xe1 && bytes.HasPrefix(data[at+4:at+2+size], []byte("Exif\x00\x00")) {
			return true
		}
		at += size + 2
	}
	return false
}

type boundedImageWriter struct {
	bytes.Buffer
	ctx       context.Context
	remaining int64
}

func (w *boundedImageWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("%w: normalized image bytes", ErrLimit)
	}
	w.remaining -= int64(len(p))
	return w.Buffer.Write(p)
}
