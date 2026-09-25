package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/pdftext"
	"regexp"
	"strconv"
	"strings"
)

const (
	pdfFormat           = "pdf"
	pdfTooLarge         = "PDF_TOO_LARGE"
	pdfMarksUnavailable = "PDF_MARKS_UNAVAILABLE"
	pageFurniture       = "ANCILLARY_CONTENT_REQUIRES_REVIEW"
	maxPDFLines         = 20000
)

type pdfInventory struct {
	Version string `json:"version"`
	Pages   int    `json:"pages"`
	Lines   int    `json:"lines"`
}

func (p ImportProcessing) extractPDF(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	if p.PDF == nil {
		return ports.StageOutput{}, worker.Failure{Code: "SOURCE_UNSUPPORTED"}
	}
	if in.Bytes < 1 || in.Bytes > domain.MaxSourceBytes {
		return ports.StageOutput{}, worker.Failure{Code: pdfTooLarge}
	}
	data := make([]byte, in.Bytes)
	if _, err := io.ReadFull(io.NewSectionReader(in.Body, 0, in.Bytes), data); err != nil {
		return ports.StageOutput{}, fmt.Errorf("read pdf source: %w", err)
	}
	doc, err := p.PDF.Read(ctx, data, pdftext.DefaultLimits())
	if err != nil {
		return ports.StageOutput{}, pdfFailure(ctx, err)
	}
	if len(doc.Lines) > maxPDFLines {
		return ports.StageOutput{}, worker.Failure{Code: pdfTooLarge}
	}
	return stageExtraction(ctx, p.WorkDir, in, extracted[pdftext.Line]{
		component: p.ExtractionVersion(in.Format), version: pdftext.Version, identity: in.Identity, mainPart: pdfFormat,
		evidence: PDFEvidence(doc, in.Identity, in.Role), blocks: doc.Lines,
		inventory: pdfInventory{Version: pdftext.Version, Pages: doc.Pages, Lines: len(doc.Lines)},
	})
}

func pdfFailure(ctx context.Context, err error) error {
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, pdftext.ErrLimit):
		return worker.Failure{Code: pdfTooLarge}
	case errors.Is(err, pdftext.ErrEncrypted):
		return worker.Failure{Code: "PDF_PROTECTED"}
	case errors.Is(err, pdftext.ErrNoText):
		return worker.Failure{Code: "PDF_NO_TEXT"}
	case errors.Is(err, pdftext.ErrInvalid):
		return worker.Failure{Code: "PDF_INVALID"}
	default:
		return err
	}
}

// PDFEvidence projects a PDF's text into recognition evidence: one paragraph per
// line. Lines that repeat at the top or bottom of the pages, and page numbers,
// are kept out of the exam and reported as page furniture. A PDF carries no
// reliable underline, bold or colour marks, which the evidence says once.
func PDFEvidence(doc pdftext.Document, sourceID, role string) domain.EvidenceDocument {
	out := domain.EvidenceDocument{SourceID: sourceID, Role: role, Version: pdftext.Version,
		Findings: []domain.EvidenceFinding{{Code: pdfMarksUnavailable, Main: true}},
		Blocks:   make([]domain.EvidenceBlock, 0, len(doc.Lines))}
	furniture := pageFurnitureOf(doc)
	line := 0
	for i, l := range doc.Lines {
		if i > 0 && doc.Lines[i-1].Page != l.Page {
			line = 0
		}
		line++
		b := domain.EvidenceBlock{ID: fmt.Sprintf("p%d-l%d", l.Page, line), Kind: "paragraph", Main: true, Meaningful: true, Safe: true, Text: l.Text, Spans: []domain.EvidenceSpan{}, Reasons: []string{}}
		if furniture[i] {
			b.Main, b.Safe, b.Reasons = false, false, []string{pageFurniture}
		}
		out.Blocks = append(out.Blocks, b)
	}
	return out
}

const edgeLines = 2

var (
	digitRun = regexp.MustCompile(`[0-9]+`)
	numbered = regexp.MustCompile(`(?i)^\s*(?:câu|question|bài)?\s*[0-9]+\s*[.):]`)
)

var pageNumber = regexp.MustCompile(`^(?:[-–—]\s*)?(?:(?:trang|page)\s*)?#(?:\s*(?:/|of|trên)\s*#)?(?:\s*[-–—])?$`)

func pageFurnitureOf(doc pdftext.Document) []bool {
	pages := pageRanges(doc.Lines)
	counts := map[string]int{}
	for _, r := range pages {
		for _, i := range edges(r) {
			counts[edgeKey(i, r, doc.Lines[i])]++
		}
	}
	out := make([]bool, len(doc.Lines))
	for _, r := range pages {
		for _, i := range fromEdge(r[0], r[1], 1) {
			if !furnitureLine(counts, i, r, doc.Lines[i]) {
				break
			}
			out[i] = true
		}
		for _, i := range fromEdge(r[1]-1, r[0]-1, -1) {
			if !furnitureLine(counts, i, r, doc.Lines[i]) {
				break
			}
			out[i] = true
		}
	}
	return out
}

func furnitureLine(counts map[string]int, i int, page [2]int, line pdftext.Line) bool {
	if pageNumber.MatchString(shape(line.Text, -1)) {
		return true
	}
	return !numbered.MatchString(line.Text) && counts[edgeKey(i, page, line)] >= 2
}

func pageRanges(lines []pdftext.Line) [][2]int {
	var out [][2]int
	for i := range lines {
		if i == 0 || lines[i-1].Page != lines[i].Page {
			out = append(out, [2]int{i, i})
		}
		out[len(out)-1][1] = i + 1
	}
	return out
}

func edges(page [2]int) []int {
	var out []int
	for i := page[0]; i < page[1]; i++ {
		if i-page[0] < edgeLines || page[1]-i <= edgeLines {
			out = append(out, i)
		}
	}
	return out
}

func fromEdge(from, to, step int) []int {
	var out []int
	for i := from; i != to && len(out) < edgeLines; i += step {
		out = append(out, i)
	}
	return out
}

func edgeKey(i int, page [2]int, line pdftext.Line) string {
	side := "top"
	if i-page[0] >= edgeLines {
		side = "bottom"
	}
	return side + "\x00" + shape(line.Text, line.Page)
}

func shape(text string, page int) string {
	fields := strings.Fields(strings.ToLower(text))
	return digitRun.ReplaceAllStringFunc(strings.Join(fields, " "), func(digits string) string {
		if n, err := strconv.Atoi(digits); page < 0 || (err == nil && n == page) {
			return "#"
		}
		return digits
	})
}
