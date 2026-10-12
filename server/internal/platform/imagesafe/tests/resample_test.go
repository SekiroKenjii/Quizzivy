package imagesafe_test

import (
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

func TestEachOutputPixelIsTheExactMeanOfItsFootprint(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 5))
	source := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	blocks := make([]color.NRGBA, 256*256)
	for by := range 256 {
		for bx := range 256 {
			c := color.NRGBA{R: uint8(random.IntN(256)), G: uint8(random.IntN(256)), B: uint8(random.IntN(256)), A: 255}
			blocks[by*256+bx] = c
			fill(source, image.Rect(2*bx, 2*by, 2*bx+2, 2*by+2), c)
		}
	}

	out := mustSquare(t, encodePNG(t, source))

	for by := range 256 {
		for bx := range 256 {
			if got, want := at(out, bx, by), blocks[by*256+bx]; got != want {
				t.Fatalf("pixel (%d,%d) is %v, want its block's %v", bx, by, got, want)
			}
		}
	}
}

func TestAnEvenMixOfBlackAndWhiteRoundsToTheMiddle(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := range 512 {
		for x := range 512 {
			v := uint8(0)
			if (x+y)%2 == 0 {
				v = 255
			}
			source.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
		}
	}

	out := mustSquare(t, encodePNG(t, source))

	for _, spot := range [][2]int{{0, 0}, {100, 37}, {255, 255}} {
		if got := at(out, spot[0], spot[1]); got.R != 128 || got.G != 128 || got.B != 128 {
			t.Errorf("pixel %v is %v, want 128 grey", spot, got)
		}
	}
}

func TestAScaleThatIsNotAWholeNumberKeepsTheMeanAndTheOrder(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 300, 300))
	var inputSum int
	for y := range 300 {
		for x := range 300 {
			v := uint8(x * 255 / 299)
			source.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 255})
			inputSum += int(v)
		}
	}

	out := mustSquare(t, encodePNG(t, source))

	var outputSum int
	previous := -1
	for x := range 256 {
		v := int(at(out, x, 100).R)
		if v < previous {
			t.Fatalf("column %d is %d after %d: the gradient reversed", x, v, previous)
		}
		previous = v
		for y := range 256 {
			outputSum += int(at(out, x, y).R)
		}
	}
	inputMean := float64(inputSum) / (300 * 300)
	outputMean := float64(outputSum) / (256 * 256)
	if diff := inputMean - outputMean; diff > 1 || diff < -1 {
		t.Errorf("the mean moved from %.2f to %.2f", inputMean, outputMean)
	}
}

func TestAWideOrTallImageIsCroppedToItsCentreNeverStretched(t *testing.T) {
	wide := image.NewNRGBA(image.Rect(0, 0, 600, 200))
	fill(wide, image.Rect(0, 0, 200, 200), red)
	fill(wide, image.Rect(200, 0, 400, 200), green)
	fill(wide, image.Rect(400, 0, 600, 200), blue)
	tall := image.NewNRGBA(image.Rect(0, 0, 200, 600))
	fill(tall, image.Rect(0, 0, 200, 200), red)
	fill(tall, image.Rect(0, 200, 200, 400), green)
	fill(tall, image.Rect(0, 400, 200, 600), blue)

	for name, source := range map[string]*image.NRGBA{"wide": wide, "tall": tall} {
		out := mustSquare(t, encodePNG(t, source))
		for _, spot := range [][2]int{{0, 0}, {255, 0}, {0, 255}, {255, 255}, {128, 128}} {
			if got := at(out, spot[0], spot[1]); got != green {
				t.Errorf("%s: pixel %v is %v, want the centre's green", name, spot, got)
			}
		}
	}
}

func TestASmallImageIsScaledUpToTheSameSize(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	fill(source, image.Rect(0, 0, 100, 200), red)
	fill(source, image.Rect(100, 0, 200, 200), blue)

	out := mustSquare(t, encodePNG(t, source))

	if at(out, 0, 0) != red || at(out, 255, 255) != blue || at(out, 127, 100) != red || at(out, 128, 100) != blue {
		t.Errorf("corners %v and %v, middle %v and %v", at(out, 0, 0), at(out, 255, 255), at(out, 127, 100), at(out, 128, 100))
	}
}

func TestColourUnderTransparencyDoesNotBleedIntoTheEdge(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 300, 300))
	fill(source, image.Rect(0, 0, 100, 300), color.NRGBA{A: 255})
	fill(source, image.Rect(100, 0, 300, 300), color.NRGBA{R: 255, A: 0})

	out := mustSquare(t, encodePNG(t, source))

	edge := at(out, 85, 100)
	if edge.A < 60 || edge.A > 110 || edge.R > 3 || edge.G > 3 || edge.B > 3 {
		t.Errorf("the edge pixel is %v, want a third opaque black and no red from under the transparent area", edge)
	}
	if hidden := at(out, 200, 100); hidden != (color.NRGBA{}) {
		t.Errorf("the transparent area is %v, want nothing behind it", hidden)
	}
}

func TestEveryKindOfSourceImageGivesTheSameColour(t *testing.T) {
	want := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	gray := image.NewGray(image.Rect(0, 0, 300, 300))
	for i := range gray.Pix {
		gray.Pix[i] = 90
	}
	paletted := image.NewPaletted(image.Rect(0, 0, 300, 300), color.Palette{want})
	wide := image.NewNRGBA64(image.Rect(0, 0, 300, 300))
	for y := range 300 {
		for x := range 300 {
			wide.SetNRGBA64(x, y, color.NRGBA64{R: 200 * 257, G: 100 * 257, B: 50 * 257, A: 0xffff})
		}
	}
	sources := map[string]image.Image{
		"nrgba":    solid(300, 300, want),
		"nrgba64":  wide,
		"paletted": paletted,
		"gray":     gray,
	}
	for name, source := range sources {
		out := mustSquare(t, encodePNG(t, source))
		expected := want
		if name == "gray" {
			expected = color.NRGBA{R: 90, G: 90, B: 90, A: 255}
		}
		if got := at(out, 128, 128); got != expected {
			t.Errorf("%s: the centre is %v, want %v", name, got, expected)
		}
	}
}
