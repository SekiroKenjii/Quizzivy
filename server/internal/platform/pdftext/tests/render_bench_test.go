package pdftext_test

import (
	"context"
	"testing"

	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/pdftext/tests/scanpdf"
)

func benchmarkRender(b *testing.B, size scanpdf.Size) {
	data := scanpdf.BuildSized(40, size)
	var own pdftext.Reader
	b.Cleanup(func() { _ = own.Close() })
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		pages, err := own.Render(context.Background(), data, pdftext.DefaultRenderLimits())
		if err != nil || len(pages) != 40 {
			b.Fatalf("%d pages, %v", len(pages), err)
		}
		var bytes int
		for _, page := range pages {
			bytes += len(page.JPEG)
		}
		b.ReportMetric(float64(bytes)/(1<<20), "jpeg-MiB")
	}
}

func BenchmarkRenderFortyPagesScannedAt300DPI(b *testing.B) {
	benchmarkRender(b, scanpdf.Size{Width: 2480, Height: 3508})
}

func BenchmarkRenderFortyPhotographsOf12Megapixels(b *testing.B) {
	benchmarkRender(b, scanpdf.Size{Width: 4000, Height: 3000, Color: true})
}
