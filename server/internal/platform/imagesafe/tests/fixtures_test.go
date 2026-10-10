package imagesafe_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"quizzivy/internal/platform/imagesafe"
	"testing"
)

var avatarLimits = imagesafe.Limits{
	MaxFileBytes:    2 << 20,
	MinSide:         200,
	MaxSide:         2048,
	OutSide:         256,
	MaxDecodedBytes: imagesafe.MaxDecodedBytes,
	MaxScans:        imagesafe.MaxScans,
}

func square(t *testing.T, data []byte) ([]byte, error) {
	t.Helper()
	return imagesafe.New(imagesafe.NewGate(2)).Square(context.Background(), bytes.NewReader(data), avatarLimits)
}

func mustSquare(t *testing.T, data []byte) *image.NRGBA {
	t.Helper()
	out, err := square(t, data)
	if err != nil {
		t.Fatalf("Square: %v", err)
	}
	return decodeOutput(t, out)
}

func decodeOutput(t *testing.T, out []byte) *image.NRGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("the output is not a png: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("the output is %v, want 256 x 256", img.Bounds())
	}
	nrgba := image.NewNRGBA(img.Bounds())
	for y := range 256 {
		for x := range 256 {
			nrgba.Set(x, y, img.At(x, y))
		}
	}
	return nrgba
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func fill(img *image.NRGBA, r image.Rectangle, c color.NRGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

var (
	red    = color.NRGBA{R: 255, A: 255}
	green  = color.NRGBA{G: 255, A: 255}
	blue   = color.NRGBA{B: 255, A: 255}
	yellow = color.NRGBA{R: 255, G: 255, A: 255}
)

func quadrants(size int, a, b, c, d color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	half := size / 2
	fill(img, image.Rect(0, 0, half, half), a)
	fill(img, image.Rect(half, 0, size, half), b)
	fill(img, image.Rect(0, half, half, size), c)
	fill(img, image.Rect(half, half, size, size), d)
	return img
}

func near(got color.NRGBA, want color.NRGBA) bool {
	within := func(a, b uint8) bool { return int(a)-int(b) < 40 && int(b)-int(a) < 40 }
	return within(got.R, want.R) && within(got.G, want.G) && within(got.B, want.B) && got.A == 255
}

func at(img *image.NRGBA, x, y int) color.NRGBA {
	return img.NRGBAAt(x, y)
}

func pngChunk(kind string, data []byte) []byte {
	chunk := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	chunk = append(chunk, kind...)
	chunk = append(chunk, data...)
	return binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(append([]byte(kind), data...)))
}

func chunkKinds(t *testing.T, data []byte) []string {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("not a png")
	}
	var kinds []string
	for pos := 8; pos < len(data); {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		kinds = append(kinds, string(data[pos+4:pos+8]))
		pos += 12 + length
	}
	return kinds
}

func withChunkAfterHeader(t *testing.T, data []byte, chunk []byte) []byte {
	t.Helper()
	const afterHeader = 8 + 12 + 13
	return append(append(append([]byte(nil), data[:afterHeader]...), chunk...), data[afterHeader:]...)
}

func declaredPNG(width, height uint32) []byte {
	header := binary.BigEndian.AppendUint32(nil, width)
	header = binary.BigEndian.AppendUint32(header, height)
	header = append(header, 8, 6, 0, 0, 0)
	return append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", header)...)
}

type frameSpec struct {
	progressive bool
	width       int
	height      int
	sampling    [][2]int
}

func jpegFrame(spec frameSpec) []byte {
	out := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	marker := byte(0xc0)
	if spec.progressive {
		marker = 0xc2
	}
	length := 8 + 3*len(spec.sampling)
	out = append(out, 0xff, marker, byte(length>>8), byte(length), 8)
	out = binary.BigEndian.AppendUint16(out, uint16(spec.height))
	out = binary.BigEndian.AppendUint16(out, uint16(spec.width))
	out = append(out, byte(len(spec.sampling)))
	for i, hv := range spec.sampling {
		out = append(out, byte(i+1), byte(hv[0]<<4|hv[1]), 0)
	}
	return out
}

func tiff(orientation uint16, bigEndian bool) []byte {
	var order binary.AppendByteOrder = binary.LittleEndian
	label := "II"
	if bigEndian {
		order, label = binary.BigEndian, "MM"
	}
	out := []byte(label)
	out = order.AppendUint16(out, 42)
	out = order.AppendUint32(out, 8)
	out = order.AppendUint16(out, 1)
	out = order.AppendUint16(out, 0x0112)
	out = order.AppendUint16(out, 3)
	out = order.AppendUint32(out, 1)
	out = order.AppendUint16(out, orientation)
	out = order.AppendUint16(out, 0)
	return order.AppendUint32(out, 0)
}

func withSegment(t *testing.T, data []byte, marker byte, body []byte) []byte {
	t.Helper()
	if data[0] != 0xff || data[1] != 0xd8 {
		t.Fatal("not a jpeg")
	}
	length := len(body) + 2
	if length > 0xffff {
		t.Fatalf("segment of %d bytes", length)
	}
	segment := append([]byte{0xff, marker, byte(length >> 8), byte(length)}, body...)
	return append(append(append([]byte(nil), data[:2]...), segment...), data[2:]...)
}

func withExif(t *testing.T, data []byte, tiffBlock []byte) []byte {
	t.Helper()
	return withSegment(t, data, 0xe1, append([]byte("Exif\x00\x00"), tiffBlock...))
}

type countingReader struct {
	r    io.Reader
	read int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += n
	return n, err
}

type endless struct{}

func (endless) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
