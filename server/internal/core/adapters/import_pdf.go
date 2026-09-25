package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/platform/pdftext"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	pdfFormat           = "pdf"
	pdfTooLarge         = "PDF_TOO_LARGE"
	pdfNoText           = "PDF_NO_TEXT"
	pdfMarksUnavailable = "PDF_MARKS_UNAVAILABLE"
	pageNumberLine      = "ANCILLARY_CONTENT_REQUIRES_REVIEW"
	repeatedLine        = "PDF_REPEATED_LINE_REQUIRES_REVIEW"
	sideBySide          = "PDF_COLUMNS_REQUIRE_REVIEW"
	maxPDFLines         = 20000
	minExamRunesPerPage = 20
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
	evidence := PDFEvidence(doc, in.Identity, in.Role)
	if !readableText(evidence, doc.Pages) {
		return ports.StageOutput{}, worker.Failure{Code: pdfNoText}
	}
	return stageExtraction(ctx, p.WorkDir, in, extracted[pdftext.Line]{
		component: p.ExtractionVersion(in.Format), version: pdftext.Version, identity: in.Identity, mainPart: pdfFormat,
		evidence: evidence, blocks: doc.Lines,
		inventory: pdfInventory{Version: pdftext.Version, Pages: doc.Pages, Lines: len(doc.Lines)},
	})
}

func readableText(e domain.EvidenceDocument, pages int) bool {
	runes := 0
	for _, b := range e.Blocks {
		if b.Main {
			runes += utf8.RuneCountInString(strings.Join(strings.Fields(b.Text), ""))
		}
	}
	if e.Role == "exam" {
		return runes >= minExamRunesPerPage*max(pages, 1)
	}
	return runes > 0
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
		return worker.Failure{Code: pdfNoText}
	case errors.Is(err, pdftext.ErrInvalid):
		return worker.Failure{Code: "PDF_INVALID"}
	default:
		return err
	}
}

// PDFEvidence projects a PDF's text into recognition evidence: one paragraph per
// line. Page numbers, and lines that repeat at the top or bottom of the pages,
// are kept out of the exam; a repeated line is reported for the teacher to
// confirm, since it may be a heading the exam needs. An exam line that holds two
// questions side by side, as a two-column page does, is flagged for review. A
// PDF carries no reliable underline, bold or colour marks, which the evidence
// says once.
func PDFEvidence(doc pdftext.Document, sourceID, role string) domain.EvidenceDocument {
	out := domain.EvidenceDocument{SourceID: sourceID, Role: role, Version: pdftext.Version,
		Findings: []domain.EvidenceFinding{{Code: pdfMarksUnavailable, Main: true}},
		Blocks:   make([]domain.EvidenceBlock, 0, len(doc.Lines))}
	hidden := pageFurnitureOf(doc)
	line := 0
	for i, l := range doc.Lines {
		if i > 0 && doc.Lines[i-1].Page != l.Page {
			line = 0
		}
		line++
		b := domain.EvidenceBlock{ID: fmt.Sprintf("p%d-l%d", l.Page, line), Kind: "paragraph", Main: true, Meaningful: true, Safe: true, Text: l.Text, Spans: []domain.EvidenceSpan{}, Reasons: []string{}}
		switch {
		case hidden[i] != "":
			b.Main, b.Safe, b.Reasons = false, false, []string{hidden[i]}
		case role == "exam" && questionsSideBySide(l.Text):
			b.Reasons = []string{sideBySide}
		}
		out.Blocks = append(out.Blocks, b)
	}
	return out
}

func questionsSideBySide(text string) bool {
	chunks := strings.Split(text, "\t")
	for _, chunk := range chunks[1:] {
		if rest, ok := recognition.QuestionStart(chunk); ok && len(strings.Fields(rest)) >= 2 {
			return true
		}
	}
	return false
}

const edgeLines = 2

var (
	digitRun   = regexp.MustCompile(`[0-9]+`)
	pageNumber = regexp.MustCompile(`^(?:[-–—]\s*)?(?:(?:trang|page)\s*)?#(?:\s*(?:/|of|trên)\s*#)?(?:\s*[-–—])?$`)
)

type edgeLine struct {
	index, page int
	top, bottom bool
	shape       string
	numbers     []int
}

func pageFurnitureOf(doc pdftext.Document) []string {
	edge := edgeLinesOf(doc)
	reasons := make(map[int]string, len(edge))
	for _, e := range edge {
		reasons[e.index] = furniture(e, edge, doc)
	}
	out := make([]string, len(doc.Lines))
	for _, r := range pageRanges(doc.Lines) {
		hideFromEdge(out, reasons, fromEdge(r[0], r[1], 1))
		hideFromEdge(out, reasons, fromEdge(r[1]-1, r[0]-1, -1))
	}
	return out
}

func edgeLinesOf(doc pdftext.Document) []edgeLine {
	var edge []edgeLine
	for _, r := range pageRanges(doc.Lines) {
		for i := r[0]; i < r[1]; i++ {
			top, bottom := i-r[0] < edgeLines, r[1]-i <= edgeLines
			if top || bottom {
				shape, numbers := tokens(doc.Lines[i].Text)
				edge = append(edge, edgeLine{index: i, page: doc.Lines[i].Page, top: top, bottom: bottom, shape: shape, numbers: numbers})
			}
		}
	}
	return edge
}

func hideFromEdge(out []string, reasons map[int]string, inward []int) {
	for _, i := range inward {
		if reasons[i] == "" {
			return
		}
		out[i] = reasons[i]
	}
}

func furniture(e edgeLine, edge []edgeLine, doc pdftext.Document) string {
	text := doc.Lines[e.index].Text
	if isPageNumber(e, doc.Pages) {
		return pageNumberLine
	}
	if _, ok := recognition.QuestionStart(text); ok || recognition.SectionStart(text) {
		return ""
	}
	for _, other := range edge {
		if other.page != e.page && ((e.top && other.top) || (e.bottom && other.bottom)) && sameRunningLine(e, other) {
			return repeatedLine
		}
	}
	return ""
}

func isPageNumber(e edgeLine, pages int) bool {
	if !pageNumber.MatchString(e.shape) || len(e.numbers) == 0 || e.numbers[0] != e.page {
		return false
	}
	return len(e.numbers) == 1 || e.numbers[1] == pages
}

func sameRunningLine(a, b edgeLine) bool {
	if a.shape != b.shape || len(a.numbers) != len(b.numbers) {
		return false
	}
	for i := range a.numbers {
		if a.numbers[i] != b.numbers[i] && (a.numbers[i] != a.page || b.numbers[i] != b.page) {
			return false
		}
	}
	return true
}

func tokens(text string) (string, []int) {
	folded := strings.ToLower(strings.Join(strings.Fields(text), " "))
	var numbers []int
	for _, digits := range digitRun.FindAllString(folded, -1) {
		n, err := strconv.Atoi(digits)
		if err != nil {
			n = -1
		}
		numbers = append(numbers, n)
	}
	return digitRun.ReplaceAllString(folded, "#"), numbers
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

func fromEdge(from, to, step int) []int {
	var out []int
	for i := from; i != to && len(out) < edgeLines; i += step {
		out = append(out, i)
	}
	return out
}
