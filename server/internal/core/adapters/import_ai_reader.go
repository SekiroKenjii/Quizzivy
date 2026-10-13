package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/reader"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/pdftext"
)

const (
	aiPageRead       = "AI_PAGE_READ"
	aiPageUnreadable = "AI_PAGE_UNREADABLE"
	aiUnreadable     = "AI_UNREADABLE"
	aiReader         = "ai"
)

// PDFPages draws a PDF's pages for the AI reader with PDFium in its WebAssembly sandbox, so a hostile file can at worst fail its own drawing.
type PDFPages struct {
	PDF *pdftext.Reader
}

func (p PDFPages) Draw(ctx context.Context, pdf []byte, limit int) ([]reader.Page, error) {
	limits := pdftext.DefaultRenderLimits()
	limits.Pages = limit
	drawn, err := p.PDF.Render(ctx, pdf, limits)
	switch {
	case errors.Is(err, pdftext.ErrLimit):
		return nil, reader.ErrTooLong
	case errors.Is(err, pdftext.ErrEncrypted):
		return nil, reader.ErrProtected
	case errors.Is(err, pdftext.ErrInvalid):
		return nil, reader.ErrUndrawable
	case err != nil:
		return nil, err
	}
	pages := make([]reader.Page, len(drawn))
	for i, page := range drawn {
		pages[i] = reader.Page{Number: page.Number, MediaType: "image/jpeg", Image: page.JPEG}
	}
	return pages, nil
}

func (p ImportProcessing) extractScan(ctx context.Context, in ports.DocumentInput, pdf []byte) (ports.StageOutput, error) {
	read, err := p.AI.Read(ctx, pdf, in.RequestedBy)
	if err != nil {
		return ports.StageOutput{}, scanFailure(ctx, err)
	}
	doc := pdftext.Document{Pages: read.Pages, Lines: make([]pdftext.Line, len(read.Lines))}
	for i, line := range read.Lines {
		doc.Lines[i] = pdftext.Line{Page: line.Page, Text: line.Text, Top: float64(len(read.Lines) - i)}
	}
	if len(doc.Lines) > maxPDFLines {
		return ports.StageOutput{}, worker.Failure{Code: pdfTooLarge}
	}
	evidence := AIEvidence(doc, read.Unreadable, in.Identity, in.Role)
	if !readableText(evidence, doc.Pages) {
		return ports.StageOutput{}, worker.Failure{Code: aiUnreadable}
	}
	p.logger().InfoContext(ctx, "import read with ai", "source", in.OriginalID, "role", in.Role,
		"pages", read.Pages, "unreadable", len(read.Unreadable), "credits", read.Credits,
		"summary", fmt.Sprintf("Read %d pages with AI · %d credits", read.Pages, read.Credits))
	return stageExtraction(ctx, p.WorkDir, in, extracted[pdftext.Line]{
		component: p.ExtractionVersion(in.Format), version: pdftext.Version, identity: in.Identity, mainPart: pdfFormat,
		evidence: evidence, blocks: doc.Lines,
		inventory: pdfInventory{Version: pdftext.Version, Pages: doc.Pages, Lines: len(doc.Lines), ReadWith: aiReader},
	})
}

func (p ImportProcessing) logger() *slog.Logger {
	if p.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return p.Logger
}

func scanFailure(ctx context.Context, err error) error {
	var short *reader.CreditsError
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, context.DeadlineExceeded):
		return worker.Failure{Code: pdfTooLarge}
	case errors.Is(err, reader.ErrTooLong):
		return worker.Failure{Code: "PDF_TOO_LONG_FOR_AI"}
	case errors.Is(err, reader.ErrProtected):
		return worker.Failure{Code: "PDF_PROTECTED"}
	case errors.Is(err, reader.ErrUndrawable):
		return worker.Failure{Code: "PDF_INVALID"}
	case errors.Is(err, reader.ErrPaused):
		return worker.Failure{Code: "AI_PAUSED"}
	case errors.As(err, &short):
		return worker.Failure{Code: "AI_CREDITS_EXHAUSTED"}
	case errors.Is(err, reader.ErrUnavailable):
		return worker.Failure{Code: "PROVIDER_UNAVAILABLE"}
	default:
		return err
	}
}

// AIEvidence projects a PDF read by the AI assistant into recognition evidence the way PDFEvidence projects a text layer, and adds what the reader owes the teacher: an informational finding for each page that was read, and a review finding on each page in unreadable, which holds a hidden marker block in the page's place so the finding can point at it. Every line stays untrusted: the recognizer validates it as it does a text layer's.
func AIEvidence(doc pdftext.Document, unreadable []int, sourceID, role string) domain.EvidenceDocument {
	out := PDFEvidence(doc, sourceID, role)
	lost := make(map[int]bool, len(unreadable))
	for _, page := range unreadable {
		lost[page] = true
	}
	blocks := make([]domain.EvidenceBlock, 0, len(out.Blocks)+len(unreadable))
	next := 0
	for page := 1; page <= doc.Pages; page++ {
		if lost[page] {
			blocks = append(blocks, domain.EvidenceBlock{ID: fmt.Sprintf("p%d-l1", page), Kind: textParagraph, Meaningful: true, Text: fmt.Sprintf("[p.%d]", page), Spans: []domain.EvidenceSpan{}, Reasons: []string{aiPageUnreadable}})
		} else {
			out.Findings = append(out.Findings, domain.EvidenceFinding{Code: aiPageRead, Main: true})
		}
		for next < len(doc.Lines) && doc.Lines[next].Page == page {
			blocks = append(blocks, out.Blocks[next])
			next++
		}
	}
	out.Blocks = blocks
	return out
}
