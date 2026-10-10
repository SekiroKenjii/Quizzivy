package imagesafe_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"quizzivy/internal/platform/imagesafe"
	"runtime"
	"testing"
)

var (
	ycc444 = [][2]int{{1, 1}, {1, 1}, {1, 1}}
	ycc420 = [][2]int{{2, 2}, {1, 1}, {1, 1}}
	cmyk   = [][2]int{{1, 1}, {1, 1}, {1, 1}, {1, 1}}
)

func TestAnImageThatWouldNeedTooMuchMemoryIsRefusedBeforeItIsDecoded(t *testing.T) {
	cases := []struct {
		name string
		spec frameSpec
		want error
	}{
		{"progressive 4:4:4 at 2048 x 2048", frameSpec{true, 2048, 2048, ycc444}, imagesafe.ErrDimensions},
		{"progressive 4:4:4 at 1850 x 1850", frameSpec{true, 1850, 1850, ycc444}, imagesafe.ErrDimensions},
		{"progressive 4:4:4 at 1800 x 1800", frameSpec{true, 1800, 1800, ycc444}, imagesafe.ErrUnreadable},
		{"progressive 4:2:0 at 2048 x 2048", frameSpec{true, 2048, 2048, ycc420}, imagesafe.ErrUnreadable},
		{"baseline 4:4:4 at 2048 x 2048", frameSpec{false, 2048, 2048, ycc444}, imagesafe.ErrUnreadable},
		{"baseline 4:2:0 at 2048 x 2048", frameSpec{false, 2048, 2048, ycc420}, imagesafe.ErrUnreadable},
		{"baseline cmyk at 2048 x 2048", frameSpec{false, 2048, 2048, cmyk}, imagesafe.ErrUnreadable},
		{"progressive cmyk at 2048 x 2048", frameSpec{true, 2048, 2048, cmyk}, imagesafe.ErrDimensions},
		{"progressive cmyk at 1500 x 1500", frameSpec{true, 1500, 1500, cmyk}, imagesafe.ErrDimensions},
		{"progressive cmyk at 1400 x 1400", frameSpec{true, 1400, 1400, cmyk}, imagesafe.ErrUnreadable},
	}
	for _, c := range cases {
		_, err := square(t, jpegFrame(c.spec))
		if !errors.Is(err, c.want) {
			t.Errorf("%s answered %v, want %v (ErrUnreadable means the estimate let it through to a decode of a file with no scan)", c.name, err, c.want)
		}
	}
}

func TestTheMemoryBoundIsFortyEightMebibytes(t *testing.T) {
	if imagesafe.MaxDecodedBytes != 48<<20 {
		t.Errorf("MaxDecodedBytes is %d, want 48 MiB", imagesafe.MaxDecodedBytes)
	}
}

func decodeAllocation(t *testing.T, decode func() error) uint64 {
	t.Helper()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := decode(); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestTheEstimateTracksWhatTheStandardDecodersReallyAllocate(t *testing.T) {
	const side = 1024
	gray := image.NewGray(image.Rect(0, 0, side, side))
	wide := image.NewNRGBA64(image.Rect(0, 0, side, side))
	paletted := image.NewPaletted(image.Rect(0, 0, side, side), color.Palette{color.Black, color.White})
	opaque := solid(side, side, red)
	translucent := solid(side, side, color.NRGBA{R: 1, G: 2, B: 3, A: 200})
	decodePNG := func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }
	transparent := pngChunk("tRNS", []byte{0, 128})
	cases := []struct {
		name   string
		data   []byte
		decode func([]byte) error
		low    float64
	}{
		{"an interlaced rgba png", handPNG(t, side, side, 6, 8, true), decodePNG, 0.7},
		{"an interlaced 16-bit rgba png", handPNG(t, side, side, 6, 16, true), decodePNG, 0.7},
		{"an interlaced gray png", handPNG(t, side, side, 0, 8, true), decodePNG, 0.7},
		{"a gray png with a tRNS chunk", handPNG(t, side, side, 0, 8, false, transparent), decodePNG, 0},
		{"a 16-bit gray png with a tRNS chunk", handPNG(t, side, side, 0, 16, false, transparent), decodePNG, 0},
		{"an rgb png with a tRNS chunk", handPNG(t, side, side, 2, 8, false, pngChunk("tRNS", []byte{0, 1, 0, 2, 0, 3})), decodePNG, 0},
		{"an interlaced gray png with a tRNS chunk", handPNG(t, side, side, 0, 8, true, transparent), decodePNG, 0.7},
		{"a gray png", encodePNG(t, gray), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }, 0},
		{"a 16-bit png", encodePNG(t, wide), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }, 0},
		{"a paletted png", encodePNG(t, paletted), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }, 0},
		{"an opaque png", encodePNG(t, opaque), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }, 0},
		{"a translucent png", encodePNG(t, translucent), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }, 0},
		{"a 4:2:0 jpeg", encodeJPEG(t, opaque), func(b []byte) error { _, err := jpeg.Decode(bytes.NewReader(b)); return err }, 0},
		{"a gray jpeg", encodeJPEG(t, gray), func(b []byte) error { _, err := jpeg.Decode(bytes.NewReader(b)); return err }, 0},
	}
	for _, c := range cases {
		actual := int64(decodeAllocation(t, func() error { return c.decode(c.data) }))
		low := c.low
		if low == 0 {
			low = 0.8
		}
		for _, probe := range []struct {
			factor float64
			want   error
		}{{low, imagesafe.ErrDimensions}, {1.6, nil}} {
			lim := avatarLimits
			lim.MaxDecodedBytes = int64(float64(actual) * probe.factor)
			_, err := imagesafe.New(imagesafe.NewGate(1)).Square(context.Background(), bytes.NewReader(c.data), lim)
			if !errors.Is(err, probe.want) {
				t.Errorf("%s allocated %d bytes: with a bound of %.1fx that, Square answered %v, want %v", c.name, actual, probe.factor, err, probe.want)
			}
		}
	}
}

func TestAnInterlacedOrTransparentPngIsCountedAsWhatTheDecoderMakesOfIt(t *testing.T) {
	const side = 400
	pixels := int64(side * side)
	transparent := pngChunk("tRNS", []byte{0, 128})
	cases := []struct {
		name string
		data []byte
		need int64
	}{
		{"a gray png", handPNG(t, side, side, 0, 8, false), pixels},
		{"a gray png with tRNS, decoded as nrgba", handPNG(t, side, side, 0, 8, false, transparent), pixels * 4},
		{"a 16-bit gray png with tRNS, decoded as nrgba64", handPNG(t, side, side, 0, 16, false, transparent), pixels * 8},
		{"an rgba png", handPNG(t, side, side, 6, 8, false), pixels * 4},
		{"an interlaced rgba png, the image and half of it again", handPNG(t, side, side, 6, 8, true), pixels * 4 * 3 / 2},
		{"an interlaced 16-bit rgba png", handPNG(t, side, side, 6, 16, true), pixels * 8 * 3 / 2},
		{"an interlaced gray png with tRNS", handPNG(t, side, side, 0, 8, true, transparent), pixels * 4 * 3 / 2},
	}
	for _, c := range cases {
		for bound, want := range map[int64]error{c.need: nil, c.need - 1: imagesafe.ErrDimensions} {
			lim := avatarLimits
			lim.MaxDecodedBytes = bound
			_, err := imagesafe.New(imagesafe.NewGate(1)).Square(context.Background(), bytes.NewReader(c.data), lim)
			if !errors.Is(err, want) {
				t.Errorf("%s needs %d bytes: under a bound of %d, Square answered %v, want %v", c.name, c.need, bound, err, want)
			}
		}
	}
}

func TestNoPngOfTheLargestSizeNeedsMoreThanTheBound(t *testing.T) {
	transparent := pngChunk("tRNS", []byte{0, 1})
	for name, header := range map[string][]byte{
		"an interlaced 16-bit rgba png":       headerOnlyPNG(2048, 2048, 6, 16, true),
		"an interlaced 16-bit gray with tRNS": headerOnlyPNG(2048, 2048, 0, 16, true, transparent),
		"an interlaced 16-bit rgb with tRNS":  headerOnlyPNG(2048, 2048, 2, 16, true, pngChunk("tRNS", make([]byte, 6))),
		"a 16-bit rgba png":                   headerOnlyPNG(2048, 2048, 6, 16, false),
	} {
		if _, err := square(t, header); !errors.Is(err, imagesafe.ErrUnreadable) {
			t.Errorf("%s answered %v, want it admitted (ErrUnreadable: the header passed and the pixels are missing)", name, err)
		}
	}
}
