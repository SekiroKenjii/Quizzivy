package adapters_test

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"strings"
	"testing"

	"quizzivy/internal/core/adapters"
	orgdomain "quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/platform/imagesafe"
)

func logoChecker() adapters.LogoChecker {
	return adapters.LogoChecker{Images: imagesafe.New(imagesafe.NewGate(2))}
}

const drawing = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300" viewBox="0 0 300 300"><rect width="300" height="300" fill="#112233"/></svg>`

func TestASquarePNGIsStoredAsANewPNG(t *testing.T) {
	file, err := logoChecker().Check(context.Background(), bytes.NewReader(photoUpload(t, 512, 512)))
	if err != nil {
		t.Fatal(err)
	}
	if file.Type != orgdomain.LogoPNG || file.Width != 512 || file.Height != 512 {
		t.Errorf("file %s %dx%d", file.Type, file.Width, file.Height)
	}
	if _, err := png.Decode(bytes.NewReader(file.Bytes)); err != nil {
		t.Errorf("the stored bytes are not a png: %v", err)
	}
}

func TestAnSVGOfDrawingElementsIsStoredAsItCame(t *testing.T) {
	file, err := logoChecker().Check(context.Background(), strings.NewReader(drawing))
	if err != nil {
		t.Fatal(err)
	}
	if file.Type != orgdomain.LogoSVG || string(file.Bytes) != drawing {
		t.Errorf("file %s %q", file.Type, file.Bytes)
	}
}

func TestEachWayToRefuseAnUploadHasItsOwnReason(t *testing.T) {
	for name, c := range map[string]struct {
		body string
		want error
	}{
		"a script in an svg":    {`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`, orgdomain.ErrSVGNotAllowed},
		"an svg of nested uses": {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><g id="g"><use href="#c"/></g><circle id="c" r="1"/><use href="#g"/><use id="u" href="#c"/><use href="#u"/></svg>`, orgdomain.ErrSVGTooComplex},
		"a gif":                 {"GIF89a\x01\x00\x01\x00", orgdomain.ErrLogoUnsupported},
		"plain text":            {"hello", orgdomain.ErrLogoUnsupported},
		"a damaged png":         {"\x89PNG\r\n\x1a\nnot really", orgdomain.ErrLogoUnreadable},
		"a png over a mebibyte": {"\x89PNG\r\n\x1a\n" + strings.Repeat("x", orgdomain.MaxPNGBytes), orgdomain.ErrLogoTooLarge},
		"an svg over 256 KiB":   {`<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat(" ", orgdomain.MaxSVGBytes) + `</svg>`, orgdomain.ErrLogoTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := logoChecker().Check(context.Background(), strings.NewReader(c.body))
			if !errors.Is(err, c.want) {
				t.Errorf("error %v, want %v", err, c.want)
			}
		})
	}
}

func TestAPNGThatIsNotSquareOrIsTooSmallIsRefusedForItsDimensions(t *testing.T) {
	for name, data := range map[string][]byte{
		"a rectangle": photoUpload(t, 512, 300),
		"too small":   photoUpload(t, 128, 128),
	} {
		if _, err := logoChecker().Check(context.Background(), bytes.NewReader(data)); !errors.Is(err, orgdomain.ErrLogoDimensions) {
			t.Errorf("%s: error %v, want ErrLogoDimensions", name, err)
		}
	}
}
