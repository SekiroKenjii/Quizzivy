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
	cases := []struct {
		name   string
		data   []byte
		decode func([]byte) error
	}{
		{"a gray png", encodePNG(t, gray), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }},
		{"a 16-bit png", encodePNG(t, wide), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }},
		{"a paletted png", encodePNG(t, paletted), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }},
		{"an opaque png", encodePNG(t, opaque), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }},
		{"a translucent png", encodePNG(t, translucent), func(b []byte) error { _, err := png.Decode(bytes.NewReader(b)); return err }},
		{"a 4:2:0 jpeg", encodeJPEG(t, opaque), func(b []byte) error { _, err := jpeg.Decode(bytes.NewReader(b)); return err }},
		{"a gray jpeg", encodeJPEG(t, gray), func(b []byte) error { _, err := jpeg.Decode(bytes.NewReader(b)); return err }},
	}
	for _, c := range cases {
		actual := int64(decodeAllocation(t, func() error { return c.decode(c.data) }))
		for _, probe := range []struct {
			factor float64
			want   error
		}{{0.8, imagesafe.ErrDimensions}, {1.6, nil}} {
			lim := avatarLimits
			lim.MaxDecodedBytes = int64(float64(actual) * probe.factor)
			_, err := imagesafe.New(imagesafe.NewGate(1)).Square(context.Background(), bytes.NewReader(c.data), lim)
			if !errors.Is(err, probe.want) {
				t.Errorf("%s allocated %d bytes: with a bound of %.1fx that, Square answered %v, want %v", c.name, actual, probe.factor, err, probe.want)
			}
		}
	}
}
