package pdftext

import (
	"context"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
)

// Page is one page of a document drawn as a JPEG: Number counts from 1.
type Page struct {
	Number        int
	JPEG          []byte
	Width, Height int
}

// RenderLimits bounds one rendering: pages, the longest side of an image in
// pixels, the JPEG quality, the most bytes one page may take, and wall time.
type RenderLimits struct {
	Pages, Edge, Quality int
	PageBytes            int64
	Timeout              time.Duration
}

// DefaultRenderLimits are 40 pages of at most 1568 pixels on the longest side,
// which is the size a vision model takes without scaling the page down, at
// JPEG quality 85, 1 MiB a page and three minutes.
func DefaultRenderLimits() RenderLimits {
	return RenderLimits{Pages: 40, Edge: 1568, Quality: 85, PageBytes: 1 << 20, Timeout: 3 * time.Minute}
}

// Render draws every page of data, in order, in the same sandbox Read uses. It
// returns ErrEncrypted, ErrLimit for a document of more pages than the limit,
// or ErrInvalid.
func (r *Reader) Render(ctx context.Context, data []byte, limits RenderLimits) ([]Page, error) {
	var pages []Page
	err := r.sandboxed(ctx, limits.Timeout, func(instance pdfium.Pdfium) error {
		var err error
		pages, err = render(instance, data, limits)
		return err
	})
	if err != nil {
		return nil, err
	}
	return pages, nil
}

func render(instance pdfium.Pdfium, data []byte, limits RenderLimits) ([]Page, error) {
	doc, count, closeDoc, err := openDocument(instance, data, limits.Pages)
	if err != nil {
		return nil, err
	}
	defer closeDoc()
	pages := make([]Page, 0, count)
	for index := range count {
		drawn, err := instance.RenderToFile(&requests.RenderToFile{
			RenderPageInPixels: &requests.RenderPageInPixels{
				Page:   requests.Page{ByIndex: &requests.PageByIndex{Document: doc, Index: index}},
				Width:  limits.Edge,
				Height: limits.Edge,
			},
			OutputFormat:  requests.RenderToFileOutputFormatJPG,
			OutputTarget:  requests.RenderToFileOutputTargetBytes,
			OutputQuality: limits.Quality,
			MaxFileSize:   limits.PageBytes,
		})
		if err != nil || drawn.ImageBytes == nil {
			return nil, ErrInvalid
		}
		pages = append(pages, Page{Number: index + 1, JPEG: *drawn.ImageBytes, Width: drawn.Width, Height: drawn.Height})
	}
	return pages, nil
}
