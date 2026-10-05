package probe_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"quizzivy/internal/platform/probe"
)

func encoded(t *testing.T, width, height int, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := encode(&b, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngOf(t *testing.T, width, height int) []byte {
	return encoded(t, width, height, func(b *bytes.Buffer, m image.Image) error { return png.Encode(b, m) })
}

func jpegOf(t *testing.T, width, height int) []byte {
	return encoded(t, width, height, func(b *bytes.Buffer, m image.Image) error { return jpeg.Encode(b, m, nil) })
}

func webp(chunk string, payload []byte) []byte {
	out := []byte("RIFF\x00\x00\x00\x00WEBP" + chunk)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(payload)))
	out = append(out, payload...)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out
}

func lossy(width, height uint16) []byte {
	payload := []byte{0x10, 0x00, 0x00, 0x9d, 0x01, 0x2a}
	payload = binary.LittleEndian.AppendUint16(payload, width)
	payload = binary.LittleEndian.AppendUint16(payload, height)
	return webp("VP8 ", append(payload, make([]byte, 8)...))
}

func lossless(width, height uint32) []byte {
	payload := binary.LittleEndian.AppendUint32([]byte{0x2f}, (width-1)|(height-1)<<14)
	return webp("VP8L", append(payload, make([]byte, 8)...))
}

func extended(width, height uint32) []byte {
	w, h := width-1, height-1
	payload := []byte{0x10, 0, 0, 0, byte(w), byte(w >> 8), byte(w >> 16), byte(h), byte(h >> 8), byte(h >> 16)}
	return webp("VP8X", payload)
}

func TestImageReadsTheSizeFromEachHeader(t *testing.T) {
	for name, c := range map[string]struct {
		data          []byte
		width, height int
	}{
		"a PNG":                           {pngOf(t, 1200, 800), 1200, 800},
		"a JPEG":                          {jpegOf(t, 1024, 768), 1024, 768},
		"a lossy WebP":                    {lossy(1600, 900), 1600, 900},
		"a lossy WebP with scaling bits":  {lossy(1600|0x4000, 900|0xc000), 1600, 900},
		"a lossless WebP":                 {lossless(640, 16384), 640, 16384},
		"an extended WebP":                {extended(70000, 3), 70000, 3},
		"an extended WebP of one by one":  {extended(1, 1), 1, 1},
		"a lossless WebP of one by one":   {lossless(1, 1), 1, 1},
		"an extended WebP, largest sides": {extended(1<<24, 1<<24), 1 << 24, 1 << 24},
	} {
		width, height, ok := probe.Image(bytes.NewReader(c.data), int64(len(c.data)))
		if !ok || width != c.width || height != c.height {
			t.Errorf("%s reads as %d by %d (ok %v), want %d by %d", name, width, height, ok, c.width, c.height)
		}
	}
}

func TestImageAnswersNothingForAHeaderItCannotRead(t *testing.T) {
	var animation bytes.Buffer
	if err := gif.Encode(&animation, image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	interframe := lossy(320, 240)
	interframe[20] |= 1
	noStartCode := lossy(320, 240)
	noStartCode[23] = 0
	noSignature := lossless(320, 240)
	noSignature[20] = 0
	unknownChunk := extended(320, 240)
	copy(unknownChunk[12:16], "ALPH")

	for name, data := range map[string][]byte{
		"nothing":                           {},
		"a PNG cut inside its header":       pngOf(t, 1200, 800)[:20],
		"a JPEG cut before its frame":       jpegOf(t, 1024, 768)[:4],
		"a lossy WebP cut short":            lossy(1600, 900)[:29],
		"a lossless WebP cut short":         lossless(640, 480)[:24],
		"an extended WebP cut short":        extended(640, 480)[:29],
		"a WebP with only its RIFF header":  []byte("RIFF\x00\x00\x00\x00WEBP"),
		"a lossy WebP of no width":          lossy(0, 900),
		"a lossy WebP of no height":         lossy(1600, 0),
		"a lossy WebP that is no key frame": interframe,
		"a lossy WebP without a start code": noStartCode,
		"a lossless WebP without its mark":  noSignature,
		"a WebP that opens on another part": unknownChunk,
		"a GIF":                             animation.Bytes(),
		"an mp3":                            []byte("ID3\x04\x00\x00\x00\x00\x00\x00 and then some frames"),
	} {
		if width, height, ok := probe.Image(bytes.NewReader(data), int64(len(data))); ok || width != 0 || height != 0 {
			t.Errorf("%s reads as %d by %d (ok %v), want nothing", name, width, height, ok)
		}
	}
}
