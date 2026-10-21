package imagesafe_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"quizzivy/internal/platform/imagesafe"
	"testing"
	"time"
)

var logoLimits = imagesafe.Limits{
	MaxFileBytes:    1 << 20,
	MinSide:         256,
	MaxSide:         2048,
	MaxDecodedBytes: imagesafe.MaxDecodedBytes,
}

func logo(t *testing.T, data []byte) ([]byte, int, int, error) {
	t.Helper()
	return imagesafe.New(imagesafe.NewGate(2)).Logo(context.Background(), bytes.NewReader(data), logoLimits)
}

func mustLogo(t *testing.T, data []byte) (image.Image, int, int) {
	t.Helper()
	out, width, height, err := logo(t, data)
	if err != nil {
		t.Fatalf("Logo: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("the output is not a png: %v", err)
	}
	return img, width, height
}

func sameLook(t *testing.T, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("the output is %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				t.Fatalf("pixel (%d,%d) is %v, want %v", x, y, got.At(x, y), want.At(x, y))
			}
		}
	}
}

func TestALogoKeepsItsPixelsAndItsSize(t *testing.T) {
	original := quadrants(300, red, green, blue, yellow)

	img, width, height := mustLogo(t, encodePNG(t, original))

	if width != 300 || height != 300 {
		t.Errorf("answered %d x %d, want 300 x 300", width, height)
	}
	sameLook(t, img, original)
}

func TestALogoKeepsTheTransparencyOfAnyPngColourType(t *testing.T) {
	palette := color.Palette{color.NRGBA{}, color.NRGBA{R: 200, A: 255}, color.NRGBA{B: 100, A: 128}}
	paletted := image.NewPaletted(image.Rect(0, 0, 256, 256), palette)
	for i := range paletted.Pix {
		paletted.Pix[i] = uint8(i % 3)
	}
	deep := image.NewNRGBA64(image.Rect(0, 0, 256, 256))
	for y := range 256 {
		for x := range 256 {
			deep.SetNRGBA64(x, y, color.NRGBA64{R: uint16(x * 257), G: uint16(y * 257), B: 0xabcd, A: 0x8000 + uint16(x)*100})
		}
	}
	gray := image.NewGray(image.Rect(0, 0, 256, 256))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i)
	}
	translucent := solid(256, 256, color.NRGBA{R: 10, G: 20, B: 30, A: 90})
	for name, original := range map[string]image.Image{"palette with alpha": paletted, "16 bits": deep, "grey": gray, "translucent": translucent} {
		t.Run(name, func(t *testing.T) {
			img, _, _ := mustLogo(t, encodePNG(t, original))
			sameLook(t, img, original)
		})
	}
}

func TestALogoLosesEveryChunkButThePixels(t *testing.T) {
	secret := []byte("SECRET-GPS-POSITION-10.762622,106.660172")
	upload := encodePNG(t, quadrants(300, red, green, blue, yellow))
	upload = withChunkAfterHeader(t, upload, pngChunk("tEXt", []byte("Author\x00SECRET-AUTHOR")))
	upload = withChunkAfterHeader(t, upload, pngChunk("eXIf", append(tiff(6, false), secret...)))
	upload = withChunkAfterHeader(t, upload, pngChunk("iCCP", []byte("SECRET-PROFILE\x00\x00")))
	upload = append(upload, []byte("SECRET-TRAILING-BYTES")...)
	if got := chunkKinds(t, upload[:len(upload)-len("SECRET-TRAILING-BYTES")]); len(got) < 6 {
		t.Fatalf("the fixture holds only the chunks %v", got)
	}

	out, _, _, err := logo(t, upload)
	if err != nil {
		t.Fatalf("Logo: %v", err)
	}

	if bytes.Contains(out, []byte("SECRET")) || bytes.Contains(out, []byte("Exif")) {
		t.Error("the output still carries metadata")
	}
	for _, kind := range chunkKinds(t, out) {
		if kind != "IHDR" && kind != "IDAT" && kind != "IEND" {
			t.Errorf("the output carries a %s chunk", kind)
		}
	}
}

func TestALogoMustBeAtLeast256AndAtMost2048OnEachSide(t *testing.T) {
	for _, side := range []int{256, 2048} {
		img, width, height := mustLogo(t, encodePNG(t, solid(side, side, red)))
		if width != side || height != side || img.Bounds().Dx() != side || img.Bounds().Dy() != side {
			t.Errorf("%d x %d answered %d x %d, output %v", side, side, width, height, img.Bounds())
		}
	}
	for _, side := range []int{255, 2049, 1} {
		if _, _, _, err := logo(t, encodePNG(t, solid(side, side, red))); !errors.Is(err, imagesafe.ErrDimensions) {
			t.Errorf("%d x %d answered %v, want ErrDimensions", side, side, err)
		}
	}
}

func TestALogoMustBeSquareWithinTwoPercent(t *testing.T) {
	refused := [][2]int{{512, 400}, {400, 512}, {500, 489}, {489, 500}, {2048, 256}, {1000, 1021}}
	for _, size := range refused {
		if _, _, _, err := logo(t, encodePNG(t, solid(size[0], size[1], red))); !errors.Is(err, imagesafe.ErrDimensions) {
			t.Errorf("%d x %d answered %v, want ErrDimensions", size[0], size[1], err)
		}
	}
	accepted := [][2]int{{500, 490}, {490, 500}, {1000, 1020}, {257, 256}}
	for _, size := range accepted {
		_, width, height := mustLogo(t, encodePNG(t, solid(size[0], size[1], red)))
		if width != size[0] || height != size[1] {
			t.Errorf("%d x %d answered %d x %d, want it unchanged", size[0], size[1], width, height)
		}
	}
}

func TestAJpegIsNotALogo(t *testing.T) {
	var animated bytes.Buffer
	if err := gif.Encode(&animated, solid(300, 300, red), nil); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"a jpeg":  encodeJPEG(t, solid(300, 300, red)),
		"a gif":   animated.Bytes(),
		"an svg":  []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"/>`),
		"nothing": nil,
	}
	for name, data := range cases {
		if _, _, _, err := logo(t, data); !errors.Is(err, imagesafe.ErrUnsupported) {
			t.Errorf("%s answered %v, want ErrUnsupported", name, err)
		}
	}
}

func TestAPngWithNothingReadableBehindItsSignatureIsUnreadable(t *testing.T) {
	good := encodePNG(t, solid(300, 300, red))
	damaged := append([]byte(nil), good...)
	damaged[len(damaged)-20] ^= 0xff
	cases := map[string][]byte{
		"only the signature": []byte("\x89PNG\r\n\x1a\n"),
		"a png cut short":    good[:len(good)/2],
		"a failed checksum":  damaged,
	}
	for name, data := range cases {
		if _, _, _, err := logo(t, data); !errors.Is(err, imagesafe.ErrUnreadable) {
			t.Errorf("%s answered %v, want ErrUnreadable", name, err)
		}
	}
}

func TestALogoDeclaringThirtyThousandSquareIsRefusedBeforeAnyDecode(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := time.Now()

	_, _, _, err = imagesafe.New(gate).Logo(ctx, bytes.NewReader(declaredPNG(30000, 30000)), logoLimits)

	if !errors.Is(err, imagesafe.ErrDimensions) {
		t.Fatalf("answered %v; a decode of this file fails with ErrUnreadable, so only ErrDimensions shows the header was judged first", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Errorf("waited %v for a slot that was never needed", time.Since(started))
	}
}

func TestLogoRefusalsNeverWaitForADecodeSlot(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	processor := imagesafe.New(gate)
	refusals := map[string]struct {
		data []byte
		want error
	}{
		"a jpeg":               {encodeJPEG(t, solid(300, 300, red)), imagesafe.ErrUnsupported},
		"a small png":          {encodePNG(t, solid(100, 100, red)), imagesafe.ErrDimensions},
		"a png that is not":    {encodePNG(t, solid(512, 400, red)), imagesafe.ErrDimensions},
		"a declared bomb":      {declaredPNG(30000, 30000), imagesafe.ErrDimensions},
		"an unreadable header": {[]byte("\x89PNG\r\n\x1a\n"), imagesafe.ErrUnreadable},
	}
	for name, c := range refusals {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		started := time.Now()
		_, _, _, err := processor.Logo(ctx, bytes.NewReader(c.data), logoLimits)
		cancel()
		if !errors.Is(err, c.want) || time.Since(started) > 5*time.Second {
			t.Errorf("%s answered %v after %v, want %v at once", name, err, time.Since(started), c.want)
		}
	}
}

func TestADecodeOfALogoWaitsForASlotOfTheSharedGate(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	upload := encodePNG(t, solid(300, 300, red))
	processor := imagesafe.New(gate)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, _, err = processor.Logo(ctx, bytes.NewReader(upload), logoLimits)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("answered %v, want the wait for a slot to end with the context", err)
	}
	release()
	if _, _, _, err := processor.Logo(context.Background(), bytes.NewReader(upload), logoLimits); err != nil {
		t.Errorf("after the slot was given back, answered %v", err)
	}
}

func TestALogoFileOverTheLimitIsRefusedAfterReadingNoMoreThanOneByteOverIt(t *testing.T) {
	body := &countingReader{r: endless{}}

	_, _, _, err := imagesafe.New(imagesafe.NewGate(2)).Logo(context.Background(), body, logoLimits)

	if !errors.Is(err, imagesafe.ErrTooLarge) {
		t.Fatalf("answered %v, want ErrTooLarge", err)
	}
	if limit := int(logoLimits.MaxFileBytes) + 1; body.read > limit {
		t.Errorf("read %d bytes, want at most %d", body.read, limit)
	}
}

func TestALogoMemoryBoundIsInclusive(t *testing.T) {
	const side = 300
	upload := encodePNG(t, solid(side, side, red))
	exact := int64(side * side * 4)
	for bound, want := range map[int64]error{exact: nil, exact - 1: imagesafe.ErrDimensions} {
		lim := logoLimits
		lim.MinSide, lim.MaxDecodedBytes = 256, bound
		_, _, _, err := imagesafe.New(imagesafe.NewGate(1)).Logo(context.Background(), bytes.NewReader(upload), lim)
		if !errors.Is(err, want) {
			t.Errorf("with a bound of %d bytes for an image of %d, answered %v, want %v", bound, exact, err, want)
		}
	}
}
