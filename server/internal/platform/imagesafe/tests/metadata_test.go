package imagesafe_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"math/rand/v2"
	"quizzivy/internal/platform/imagesafe"
	"testing"
	"time"
)

func TestNothingButPixelsSurvivesInTheOutput(t *testing.T) {
	secret := []byte("SECRET-GPS-POSITION-10.762622,106.660172")
	photo := encodeJPEG(t, solid(400, 300, red))
	photo = withExif(t, photo, append(tiff(6, false), secret...))
	photo = withSegment(t, photo, 0xfe, []byte("SECRET-COMMENT"))
	photo = withSegment(t, photo, 0xed, []byte("Photoshop 3.0\x008BIM SECRET-IPTC"))
	photo = append(photo, []byte("SECRET-TRAILING-BYTES")...)

	graphic := encodePNG(t, solid(400, 300, red))
	graphic = withChunkAfterHeader(t, graphic, pngChunk("tEXt", []byte("Author\x00SECRET-AUTHOR")))
	graphic = withChunkAfterHeader(t, graphic, pngChunk("eXIf", append(tiff(6, false), secret...)))
	graphic = withChunkAfterHeader(t, graphic, pngChunk("iCCP", []byte("SECRET-PROFILE\x00\x00")))
	graphic = append(graphic, []byte("SECRET-TRAILING-BYTES")...)

	for name, upload := range map[string][]byte{"a jpeg": photo, "a png": graphic} {
		out, err := square(t, upload)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bytes.Contains(out, []byte("SECRET")) || bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("Photoshop")) {
			t.Errorf("%s: the output still carries metadata", name)
		}
		kinds := chunkKinds(t, out)
		if kinds[0] != "IHDR" || kinds[len(kinds)-1] != "IEND" {
			t.Errorf("%s: chunks %v", name, kinds)
		}
		for _, kind := range kinds {
			if kind != "IHDR" && kind != "IDAT" && kind != "IEND" {
				t.Errorf("%s: the output carries a %s chunk", name, kind)
			}
		}
	}
}

func TestAPhoneSidewaysPortraitEndsUpright(t *testing.T) {
	raw := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	fill(raw, image.Rect(0, 0, 200, 300), blue)
	fill(raw, image.Rect(200, 0, 400, 300), green)
	sideways := encodeJPEG(t, raw)

	upright := mustSquare(t, withExif(t, sideways, tiff(6, false)))
	if !near(at(upright, 128, 30), blue) || !near(at(upright, 128, 226), green) {
		t.Errorf("with orientation 6 the top is %v and the bottom %v, want blue over green", at(upright, 128, 30), at(upright, 128, 226))
	}

	untouched := mustSquare(t, sideways)
	if !near(at(untouched, 30, 128), blue) || !near(at(untouched, 226, 128), green) {
		t.Errorf("without a tag the left is %v and the right %v, want blue beside green", at(untouched, 30, 128), at(untouched, 226, 128))
	}
}

func TestEveryOrientationTurnsTheQuadrantsUpright(t *testing.T) {
	palette := []color.NRGBA{red, green, blue, yellow}
	raw := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	shown := map[uint16][4]int{
		1: {0, 1, 2, 3},
		2: {1, 0, 3, 2},
		3: {3, 2, 1, 0},
		4: {2, 3, 0, 1},
		5: {0, 2, 1, 3},
		6: {2, 0, 3, 1},
		7: {3, 1, 2, 0},
		8: {1, 3, 0, 2},
	}
	spots := [4][2]int{{64, 64}, {192, 64}, {64, 192}, {192, 192}}
	for orientation, want := range shown {
		for _, bigEndian := range []bool{false, true} {
			out := mustSquare(t, withExif(t, raw, tiff(orientation, bigEndian)))
			for i, spot := range spots {
				if got := at(out, spot[0], spot[1]); !near(got, palette[want[i]]) {
					t.Errorf("orientation %d (big endian %v): quadrant %d is %v, want %v", orientation, bigEndian, i, got, palette[want[i]])
				}
			}
		}
	}
}

func TestAnExifBlockThatIsDamagedOrOddIsIgnored(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	want, err := square(t, plain)
	if err != nil {
		t.Fatal(err)
	}
	tooMany := tiff(6, false)
	tooMany[8], tooMany[9] = 0xff, 0xff
	farAway := tiff(6, false)
	farAway[4], farAway[5], farAway[6], farAway[7] = 0xff, 0xff, 0xff, 0x7f
	beforeHeader := tiff(6, false)
	beforeHeader[4], beforeHeader[5], beforeHeader[6], beforeHeader[7] = 2, 0, 0, 0
	wrongType := tiff(6, false)
	wrongType[12] = 4
	repeated := tiff(6, false)
	repeated[14] = 2
	notTiff := tiff(6, false)
	notTiff[2] = 43
	badOrder := tiff(6, false)
	badOrder[0], badOrder[1] = 'X', 'X'
	cases := map[string][]byte{
		"orientation 0":                tiff(0, false),
		"orientation 9":                tiff(9, false),
		"a count of 65535 entries":     tooMany,
		"an IFD far outside the block": farAway,
		"an IFD inside the header":     beforeHeader,
		"a value of the wrong type":    wrongType,
		"a count other than one":       repeated,
		"a wrong magic number":         notTiff,
		"an unknown byte order":        badOrder,
		"an empty block":               nil,
		"only a byte order":            []byte("II"),
		"a header with no IFD":         {'I', 'I', 42, 0, 8, 0, 0, 0},
	}
	for name, block := range cases {
		got, err := square(t, withExif(t, plain, block))
		if err != nil {
			t.Errorf("%s answered %v, want the image as it is", name, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed the output", name)
		}
	}
}

func TestAnExifSegmentThatIsNotExifIsIgnored(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	want, err := square(t, plain)
	if err != nil {
		t.Fatal(err)
	}
	xmp := append([]byte("http://ns.adobe.com/xap/1.0/\x00"), tiff(6, false)...)
	other := append([]byte("Exiff\x00"), tiff(6, false)...)
	for name, body := range map[string][]byte{"xmp": xmp, "a near miss": other, "a short one": []byte("Exif")} {
		got, err := square(t, withSegment(t, plain, 0xe1, body))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: answered %v and changed the output: %v", name, err, !bytes.Equal(got, want))
		}
	}
}

func TestOnlyTheFirstExifSegmentCounts(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	twice := withExif(t, withExif(t, plain, tiff(6, false)), tiff(3, false))

	out := mustSquare(t, twice)

	if !near(at(out, 64, 64), yellow) {
		t.Errorf("the top left is %v, want yellow: the segment first in the file turns the image half way round", at(out, 64, 64))
	}
}

func TestAnIfdThatPointsBackAtItselfIsReadOnceAndDoesNotLoop(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	looping := tiff(6, false)
	looping[len(looping)-4] = 8
	want, err := square(t, withExif(t, plain, tiff(6, false)))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var got []byte
	go func() {
		defer close(done)
		got, err = square(t, withExif(t, plain, looping))
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a looping IFD chain hung the reader")
	}

	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("answered %v; the orientation of the first IFD should still be applied", err)
	}
}

func TestRandomlyDamagedExifNeverPanicsAndNeverBreaksTheOutput(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	random := rand.New(rand.NewPCG(7, 11))
	processor := imagesafe.New(imagesafe.NewGate(1))
	for i := range 160 {
		block := tiff(uint16(1+i%8), i%2 == 0)
		for range 1 + random.IntN(4) {
			block[random.IntN(len(block))] = byte(random.IntN(256))
		}
		if random.IntN(4) == 0 {
			block = block[:random.IntN(len(block))]
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("round %d panicked on %x: %v", i, block, r)
				}
			}()
			out, err := processor.Square(context.Background(), bytes.NewReader(withExif(t, plain, block)), avatarLimits)
			if err != nil {
				t.Fatalf("round %d answered %v for %x", i, err, block)
			}
			decodeOutput(t, out)
		}()
	}
}

func TestAJpegWhoseSegmentLengthLiesIsUnreadable(t *testing.T) {
	plain := encodeJPEG(t, quadrants(256, red, green, blue, yellow))
	broken := withExif(t, plain, tiff(6, false))
	broken[4], broken[5] = 0xff, 0xff

	if _, err := square(t, broken); !errors.Is(err, imagesafe.ErrUnreadable) {
		t.Errorf("answered %v, want ErrUnreadable", err)
	}
}

func TestTheSameUploadAlwaysGivesTheSameBytes(t *testing.T) {
	upload := withExif(t, encodeJPEG(t, quadrants(300, red, green, blue, yellow)), tiff(8, false))

	first, err := square(t, upload)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		again, err := square(t, upload)
		if err != nil || !bytes.Equal(first, again) {
			t.Fatalf("run %d differs: %v", i, err)
		}
	}
}
