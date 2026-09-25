package word_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"quizzivy/internal/platform/word"
)

func imagePackage(t testing.TB, data []byte) []byte {
	t.Helper()
	entries := baseEntries(`<w:p><w:r><w:t>Image question</w:t></w:r></w:p>`)
	entries[0].text = strings.Replace(entries[0].text, "</Types>", `<Default Extension="png" ContentType="image/png"/></Types>`, 1)
	return pack(t, append(entries, entry{"word/media/image.png", string(data)}))
}

func imagePNG(t testing.TB) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	im.Set(1, 2, color.NRGBA{R: 120, G: 50, B: 70, A: 255})
	var out bytes.Buffer
	if err := png.Encode(&out, im); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func pngChunk(name string, data []byte) []byte {
	out := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(out, uint32(len(data)))
	copy(out[4:], name)
	copy(out[8:], data)
	binary.BigEndian.PutUint32(out[len(out)-4:], crc32.ChecksumIEEE(out[4:len(out)-4]))
	return out
}

func readImage(t testing.TB, data []byte, limits word.ImageLimits) (word.NormalizedImage, error) {
	t.Helper()
	pkg := imagePackage(t, data)
	return word.ReadImage(context.Background(), bytes.NewReader(pkg), int64(len(pkg)), "word/media/image.png", word.DefaultLimits(), limits)
}

func TestEmbeddedImageIsDecodedAndReencodedWithoutPrivateMetadata(t *testing.T) {
	raw := imagePNG(t)
	withText := append([]byte{}, raw[:33]...)
	withText = append(withText, pngChunk("tEXt", []byte("Answer\x00PRIVATE_KEY"))...)
	withText = append(withText, raw[33:]...)
	got, err := readImage(t, withText, word.DefaultImageLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 2 || got.Height != 3 || bytes.Contains(got.PNG, []byte("PRIVATE_KEY")) {
		t.Fatal("normalization kept private metadata or changed dimensions")
	}
	im, err := png.Decode(bytes.NewReader(got.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if color.NRGBAModel.Convert(im.At(1, 2)) != (color.NRGBA{R: 120, G: 50, B: 70, A: 255}) {
		t.Fatal("image pixels changed")
	}
}

func TestEmbeddedImageBoundsPixelsBytesAndOutput(t *testing.T) {
	raw := imagePNG(t)
	for _, tc := range []struct {
		name   string
		limits word.ImageLimits
	}{
		{"pixels", word.ImageLimits{Bytes: 1000, Pixels: 5, Dimension: 10, OutputBytes: 1000}},
		{"dimension", word.ImageLimits{Bytes: 1000, Pixels: 100, Dimension: 2, OutputBytes: 1000}},
		{"input", word.ImageLimits{Bytes: 10, Pixels: 100, Dimension: 10, OutputBytes: 1000}},
		{"output", word.ImageLimits{Bytes: 1000, Pixels: 100, Dimension: 10, OutputBytes: 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readImage(t, raw, tc.limits); !errors.Is(err, word.ErrLimit) {
				t.Fatalf("resource bound ignored: %v", err)
			}
		})
	}
}

func TestEmbeddedImagesDoNotSilentlyLoseAnimationOrOrientation(t *testing.T) {
	raw := imagePNG(t)
	for _, chunk := range []string{"acTL", "eXIf"} {
		modified := append([]byte{}, raw[:33]...)
		modified = append(modified, pngChunk(chunk, make([]byte, 8))...)
		modified = append(modified, raw[33:]...)
		if _, err := readImage(t, modified, word.DefaultImageLimits()); !errors.Is(err, word.ErrUnsupportedImage) {
			t.Fatalf("silently flattened %s: %v", chunk, err)
		}
	}
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := readImage(t, jpegBytes.Bytes(), word.DefaultImageLimits()); err != nil {
		t.Fatal(err)
	}
	withExif := []byte{0xff, 0xd8, 0xff, 0xe1, 0, 8, 'E', 'x', 'i', 'f', 0, 0}
	withExif = append(withExif, jpegBytes.Bytes()[2:]...)
	if _, err := readImage(t, withExif, word.DefaultImageLimits()); !errors.Is(err, word.ErrUnsupportedImage) {
		t.Fatalf("ignored EXIF orientation: %v", err)
	}
}

func TestEmbeddedInvalidTruncatedAndVectorImagesRemainUnusable(t *testing.T) {
	raw := imagePNG(t)
	for _, data := range [][]byte{[]byte(`<svg onload="fetch('https://example.com')"/>`), []byte("not an image"), raw[:len(raw)-10]} {
		if _, err := readImage(t, data, word.DefaultImageLimits()); err == nil {
			t.Fatal("accepted invalid or active image")
		}
	}
	pkg := imagePackage(t, raw)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := word.ReadImage(ctx, bytes.NewReader(pkg), int64(len(pkg)), "word/media/image.png", word.DefaultLimits(), word.DefaultImageLimits()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if _, err := word.ReadImage(context.Background(), bytes.NewReader(pkg), int64(len(pkg)), "../secret", word.DefaultLimits(), word.DefaultImageLimits()); !errors.Is(err, word.ErrInvalidPackage) {
		t.Fatalf("read outside package: %v", err)
	}
}
