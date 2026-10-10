package adapters_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"quizzivy/internal/core/adapters"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/imagesafe"
	"testing"
)

func photoUpload(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 30, 120, 200, 255
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type endlessBytes struct{}

func (endlessBytes) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestPhotosMakeA256SquarePngFromAnUploadWithinTheAvatarLimits(t *testing.T) {
	photos := adapters.Photos{Processor: imagesafe.New(imagesafe.NewGate(2))}

	out, err := photos.Square(context.Background(), bytes.NewReader(photoUpload(t, 640, 480)))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("stored %v, %v; want a 256 x 256 png", img.Bounds(), err)
	}
}

func TestPhotosAnswerTheIdentityDomainsReasonForEachRefusal(t *testing.T) {
	var animated bytes.Buffer
	if err := gif.Encode(&animated, image.NewPaletted(image.Rect(0, 0, 300, 300), color.Palette{color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		body io.Reader
		want error
	}{
		{"a file over 2 MiB", endlessBytes{}, identitydomain.ErrAvatarTooLarge},
		{"a gif", bytes.NewReader(animated.Bytes()), identitydomain.ErrAvatarUnsupported},
		{"a header with nothing behind it", bytes.NewReader([]byte("\x89PNG\r\n\x1a\n")), identitydomain.ErrAvatarUnreadable},
		{"a side under 200", bytes.NewReader(photoUpload(t, 199, 400)), identitydomain.ErrAvatarDimensions},
		{"a side over 2048", bytes.NewReader(photoUpload(t, 2049, 300)), identitydomain.ErrAvatarDimensions},
	}
	photos := adapters.Photos{Processor: imagesafe.New(imagesafe.NewGate(2))}
	for _, c := range cases {
		if _, err := photos.Square(context.Background(), c.body); !errors.Is(err, c.want) {
			t.Errorf("%s answered %v, want %v", c.name, err, c.want)
		}
	}
}

func TestPhotosPassOnAReadThatFailsAsItIs(t *testing.T) {
	cause := errors.New("connection reset")
	photos := adapters.Photos{Processor: imagesafe.New(imagesafe.NewGate(2))}

	_, err := photos.Square(context.Background(), io.MultiReader(bytes.NewReader([]byte{1}), errReader{cause}))

	if !errors.Is(err, cause) {
		t.Errorf("answered %v, want the read's own error", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func jpegOfScans(scans int) []byte {
	out := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	out = append(out, 0xff, 0xc0, 0x00, 0x0b, 8, 0x01, 0x2c, 0x01, 0x2c, 1, 1, 0x11, 0)
	for range scans {
		out = append(out, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00, 0x00)
	}
	return append(out, 0xff, 0xd9)
}

func TestPhotosRefuseAJpegOfMoreThanThirtyTwoScansBeforeDecodingIt(t *testing.T) {
	photos := adapters.Photos{Processor: imagesafe.New(imagesafe.NewGate(1))}

	if _, err := photos.Square(context.Background(), bytes.NewReader(jpegOfScans(33))); !errors.Is(err, identitydomain.ErrAvatarDimensions) {
		t.Errorf("33 scans answered %v, want ErrAvatarDimensions", err)
	}
	if _, err := photos.Square(context.Background(), bytes.NewReader(jpegOfScans(32))); !errors.Is(err, identitydomain.ErrAvatarUnreadable) {
		t.Errorf("32 scans answered %v, want them let through to a decode that fails (ErrAvatarUnreadable)", err)
	}
}
