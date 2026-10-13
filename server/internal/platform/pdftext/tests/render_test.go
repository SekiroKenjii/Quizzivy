package pdftext_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"testing"
	"time"

	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/pdftext/tests/scanpdf"
)

func TestEveryPageOfAScanIsDrawnInOrderWithinTheEdge(t *testing.T) {
	limits := pdftext.DefaultRenderLimits()
	pages, err := reader.Render(context.Background(), scanpdf.Build(3), limits)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 3 {
		t.Fatalf("%d pages, want 3", len(pages))
	}
	var previous image.Image
	for i, page := range pages {
		if page.Number != i+1 {
			t.Fatalf("page %d is numbered %d", i+1, page.Number)
		}
		decoded, err := jpeg.Decode(bytes.NewReader(page.JPEG))
		if err != nil {
			t.Fatalf("page %d is not a JPEG: %v", page.Number, err)
		}
		bounds := decoded.Bounds()
		if bounds.Dx() != page.Width || bounds.Dy() != page.Height || max(page.Width, page.Height) != limits.Edge {
			t.Fatalf("page %d is %dx%d, reported %dx%d, edge %d", page.Number, bounds.Dx(), bounds.Dy(), page.Width, page.Height, limits.Edge)
		}
		if !hasInk(decoded) {
			t.Fatalf("page %d came out blank", page.Number)
		}
		if previous != nil && sameImage(previous, decoded) {
			t.Fatalf("pages %d and %d are the same picture", page.Number-1, page.Number)
		}
		previous = decoded
	}
}

func hasInk(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x4000 {
				return true
			}
		}
	}
	return false
}

func sameImage(a, b image.Image) bool {
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			ar, _, _, _ := a.At(x, y).RGBA()
			br, _, _, _ := b.At(x, y).RGBA()
			if ar>>10 != br>>10 {
				return false
			}
		}
	}
	return true
}

func TestRenderingStaysWithinItsLimits(t *testing.T) {
	limits := pdftext.DefaultRenderLimits()
	limits.Pages = 2
	if _, err := reader.Render(context.Background(), scanpdf.Build(3), limits); !errors.Is(err, pdftext.ErrLimit) {
		t.Fatalf("three pages under a limit of two: err = %v, want ErrLimit", err)
	}
	limits = pdftext.DefaultRenderLimits()
	limits.PageBytes = 200
	if _, err := reader.Render(context.Background(), scanpdf.Build(1), limits); !errors.Is(err, pdftext.ErrInvalid) {
		t.Fatalf("a page over its byte limit: err = %v, want ErrInvalid", err)
	}
	limits = pdftext.DefaultRenderLimits()
	limits.Timeout = time.Nanosecond
	if _, err := reader.Render(context.Background(), scanpdf.Build(1), limits); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an expired deadline answered %v", err)
	}
	if _, err := reader.Render(context.Background(), scanpdf.Build(1), pdftext.DefaultRenderLimits()); err != nil {
		t.Fatalf("the reader did not recover after a refusal: %v", err)
	}
}

func TestAProtectedOrBrokenDocumentIsNotDrawn(t *testing.T) {
	if _, err := reader.Render(context.Background(), build([][]run{{{72, 760, "secret", 0}}}, true), pdftext.DefaultRenderLimits()); !errors.Is(err, pdftext.ErrEncrypted) {
		t.Fatalf("a protected document: err = %v, want ErrEncrypted", err)
	}
	if _, err := reader.Render(context.Background(), []byte("%PDF-1.4\nnot really"), pdftext.DefaultRenderLimits()); !errors.Is(err, pdftext.ErrInvalid) {
		t.Fatalf("a broken document: err = %v, want ErrInvalid", err)
	}
}
