package imagesafe_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/gif"
	"quizzivy/internal/platform/imagesafe"
	"testing"
	"time"
)

func TestOnlyAPngOrAJpegIsAccepted(t *testing.T) {
	var animated bytes.Buffer
	if err := gif.Encode(&animated, solid(300, 300, red), nil); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"a gif":              animated.Bytes(),
		"a webp":             append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 64)...),
		"a bmp":              append([]byte("BM"), make([]byte, 64)...),
		"an svg":             []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="300"><script>alert(1)</script></svg>`),
		"html":               []byte("<!doctype html><title>x</title>"),
		"nothing":            nil,
		"a lone byte":        {0xff},
		"text":               []byte("hello"),
		"a zip":              append([]byte("PK\x03\x04"), make([]byte, 64)...),
		"a pdf":              []byte("%PDF-1.7\n"),
		"a truncated header": []byte("\x89PNG\r\n"),
	}
	for name, data := range cases {
		if _, err := square(t, data); !errors.Is(err, imagesafe.ErrUnsupported) {
			t.Errorf("%s answered %v, want ErrUnsupported", name, err)
		}
	}
}

func TestTheTypeComesFromTheBytesAndNotFromTheLabelOfThePart(t *testing.T) {
	jpegAsPNG := encodeJPEG(t, solid(300, 300, red))
	if _, err := square(t, jpegAsPNG); err != nil {
		t.Errorf("a jpeg answered %v", err)
	}
	pngAsJPEG := encodePNG(t, solid(300, 300, red))
	if _, err := square(t, pngAsJPEG); err != nil {
		t.Errorf("a png answered %v", err)
	}
}

func TestASignatureWithNothingReadableBehindItIsUnreadableNotUnsupported(t *testing.T) {
	good := encodePNG(t, solid(300, 300, red))
	corrupt := append([]byte(nil), good...)
	for i := 40; i < len(corrupt)-20; i++ {
		corrupt[i] ^= 0x5a
	}
	cases := map[string][]byte{
		"only the png signature":         []byte("\x89PNG\r\n\x1a\n"),
		"a png cut short":                good[:len(good)/2],
		"a png with its data gone wrong": corrupt,
		"only the jpeg signature":        {0xff, 0xd8, 0xff},
		"a jpeg cut short":               encodeJPEG(t, solid(300, 300, red))[:300],
		"a jpeg of noise":                append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0x13, 0x37}, 400)...),
	}
	for name, data := range cases {
		if _, err := square(t, data); !errors.Is(err, imagesafe.ErrUnreadable) {
			t.Errorf("%s answered %v, want ErrUnreadable", name, err)
		}
	}
}

func TestAFileOverTheLimitIsRefusedAfterReadingNoMoreThanOneByteOverIt(t *testing.T) {
	body := &countingReader{r: endless{}}

	_, err := imagesafe.New(imagesafe.NewGate(2)).Square(context.Background(), body, avatarLimits)

	if !errors.Is(err, imagesafe.ErrTooLarge) {
		t.Fatalf("answered %v, want ErrTooLarge", err)
	}
	if limit := int(avatarLimits.MaxFileBytes) + 1; body.read > limit {
		t.Errorf("read %d bytes, want at most %d", body.read, limit)
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

func TestAReadThatFailsIsReturnedAsItIsAndNotAsATooLargeFile(t *testing.T) {
	cause := errors.New("connection reset")

	_, err := imagesafe.New(imagesafe.NewGate(2)).Square(context.Background(), failingReader{cause}, avatarLimits)

	if !errors.Is(err, cause) || errors.Is(err, imagesafe.ErrTooLarge) {
		t.Errorf("answered %v, want the read's own error", err)
	}
}

func TestSidesOutsideTwoHundredToTwoThousandAndFortyEightAreRefused(t *testing.T) {
	refused := [][2]int{{199, 300}, {300, 199}, {2049, 300}, {300, 2049}, {1, 1}, {199, 199}}
	for _, size := range refused {
		if _, err := square(t, encodePNG(t, solid(size[0], size[1], red))); !errors.Is(err, imagesafe.ErrDimensions) {
			t.Errorf("%dx%d answered %v, want ErrDimensions", size[0], size[1], err)
		}
	}
	accepted := [][2]int{{200, 200}, {2048, 200}, {200, 2048}, {300, 700}}
	for _, size := range accepted {
		if _, err := square(t, encodePNG(t, solid(size[0], size[1], red))); err != nil {
			t.Errorf("%dx%d answered %v, want success", size[0], size[1], err)
		}
	}
}

func TestAPngDeclaringThirtyThousandSquareIsRefusedBeforeAnyDecode(t *testing.T) {
	bomb := declaredPNG(30000, 30000)

	_, err := square(t, bomb)

	if !errors.Is(err, imagesafe.ErrDimensions) {
		t.Fatalf("answered %v; a decode of this file fails with ErrUnreadable, so only ErrDimensions shows the header was judged first", err)
	}
}

func TestRefusalsNeverWaitForADecodeSlot(t *testing.T) {
	gate := imagesafe.NewGate(1)
	release, err := gate.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	processor := imagesafe.New(gate)
	var animated bytes.Buffer
	if err := gif.Encode(&animated, solid(300, 300, red), nil); err != nil {
		t.Fatal(err)
	}
	refusals := map[string]struct {
		data []byte
		want error
	}{
		"a gif":                {animated.Bytes(), imagesafe.ErrUnsupported},
		"a small png":          {encodePNG(t, solid(100, 100, red)), imagesafe.ErrDimensions},
		"a declared bomb":      {declaredPNG(30000, 30000), imagesafe.ErrDimensions},
		"a heavy progressive":  {jpegFrame(frameSpec{progressive: true, width: 2048, height: 2048, sampling: [][2]int{{1, 1}, {1, 1}, {1, 1}}}), imagesafe.ErrDimensions},
		"an unreadable header": {[]byte("\x89PNG\r\n\x1a\n"), imagesafe.ErrUnreadable},
	}
	for name, c := range refusals {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		started := time.Now()
		_, err := processor.Square(ctx, bytes.NewReader(c.data), avatarLimits)
		cancel()
		if !errors.Is(err, c.want) || time.Since(started) > 5*time.Second {
			t.Errorf("%s answered %v after %v, want %v at once", name, err, time.Since(started), c.want)
		}
	}
}

func TestAPngWhoseDataFailsItsChecksumIsUnreadable(t *testing.T) {
	good := encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 300, 300)))
	flipped := append([]byte(nil), good...)
	flipped[len(flipped)-20] ^= 0xff
	flipped[len(flipped)-30] ^= 0xff

	_, err := square(t, flipped)

	if !errors.Is(err, imagesafe.ErrUnreadable) {
		t.Errorf("answered %v, want ErrUnreadable", err)
	}
}

func TestAFileOfExactlyTheLimitIsAcceptedAndOneByteMoreIsNot(t *testing.T) {
	picture := encodePNG(t, solid(300, 300, red))
	exact := append(append([]byte(nil), picture...), bytes.Repeat([]byte{0}, int(avatarLimits.MaxFileBytes)-len(picture))...)

	if _, err := square(t, exact); err != nil {
		t.Errorf("a file of %d bytes answered %v, want success", len(exact), err)
	}
	if _, err := square(t, append(exact, 0)); !errors.Is(err, imagesafe.ErrTooLarge) {
		t.Errorf("a file of %d bytes answered %v, want ErrTooLarge", len(exact)+1, err)
	}
}

func TestTheMemoryBoundIsInclusive(t *testing.T) {
	const side = 400
	upload := encodePNG(t, solid(side, side, red))
	exact := int64(side * side * 4)
	for bound, want := range map[int64]error{exact: nil, exact - 1: imagesafe.ErrDimensions} {
		lim := avatarLimits
		lim.MaxDecodedBytes = bound
		_, err := imagesafe.New(imagesafe.NewGate(1)).Square(context.Background(), bytes.NewReader(upload), lim)
		if !errors.Is(err, want) {
			t.Errorf("with a bound of %d bytes for an image of %d, answered %v, want %v", bound, exact, err, want)
		}
	}
}
