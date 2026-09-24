package word

import (
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	elementMoveFrom    = "moveFrom"
	elementMoveTo      = "moveTo"
	propertyWebHidden  = "webHidden"
	elementInsertion   = "ins"
	elementObject      = "object"
	elementTable       = "tbl"
	elementInstruction = "instrText"
	propertyHidden     = "vanish"
	elementDeletion    = "del"
	elementField       = "fldSimple"
)

const fieldReview = "FIELD_REQUIRES_REVIEW"

func paragraphEvidence(p Paragraph, resolved ResolvedParagraph, contexts map[Locator]string) (TextEvidence, bool) {
	out := TextEvidence{Numbering: resolved.Numbering, Runs: []RunEvidence{}}
	runs := make(map[Locator]ResolvedRun, len(resolved.Runs))
	for _, r := range resolved.Runs {
		runs[r.Locator] = r
	}
	position, meaningful := 0, false
	for _, r := range p.Runs {
		marks := runs[r.Locator]
		next := RunEvidence{Locator: r.Locator, Contexts: r.Contexts, Complete: marks.Complete, Marks: marks.Marks, Properties: r.Properties, Fragments: []TextFragment{}}
		next.ReviewReasons = runReasons(marks)
		next.ReviewReasons = appendReasons(next.ReviewReasons, contextReasons(r.Contexts, contexts)...)
		if changedProperties(r.Properties) {
			next.ReviewReasons = appendReasons(next.ReviewReasons, "TRACKED_CHANGE_REQUIRES_REVIEW")
		}
		for _, fragment := range r.Fragments {
			end := position + utf8.RuneCountInString(fragment.Text)
			next.Fragments = append(next.Fragments, TextFragment{Fragment: fragment, Start: position, EndOffset: end})
			position = end
			meaningful = meaningful || strings.TrimSpace(fragment.Text) != ""
			if fragment.Kind == elementInstruction {
				next.ReviewReasons = appendReasons(next.ReviewReasons, fieldReview)
			}
			if fragment.Kind == "delText" {
				next.ReviewReasons = appendReasons(next.ReviewReasons, "TRACKED_CHANGE_REQUIRES_REVIEW")
			}
		}
		out.Runs = append(out.Runs, next)
	}
	return out, meaningful || resolved.Numbering != nil
}

func runReasons(r ResolvedRun) []string {
	var reasons []string
	if !r.Complete {
		reasons = append(reasons, "STYLE_REQUIRES_REVIEW")
	}
	for _, mark := range r.Marks {
		if (mark.Name == propertyHidden || mark.Name == propertyWebHidden) && (!mark.Resolved || mark.Value == "on") {
			reasons = appendReasons(reasons, "HIDDEN_TEXT_REQUIRES_REVIEW")
		}
	}
	return reasons
}

func blockReasons(b SourceBlock) []string {
	var reasons []string
	if b.PartKind != "document" {
		reasons = append(reasons, "ANCILLARY_CONTENT_REQUIRES_REVIEW")
	}
	if b.Object != nil {
		reasons = append(reasons, "OBJECT_REQUIRES_REVIEW")
	}
	if changedProperties(b.Properties) {
		reasons = appendReasons(reasons, "TRACKED_CHANGE_REQUIRES_REVIEW")
	}
	switch b.Kind {
	case elementInsertion, elementDeletion, elementMoveFrom, elementMoveTo:
		reasons = append(reasons, "TRACKED_CHANGE_REQUIRES_REVIEW")
	case elementField:
		reasons = append(reasons, fieldReview)
	case elementTextBox:
		reasons = append(reasons, "TEXTBOX_ORDER_REQUIRES_REVIEW")
	case "unassigned":
		reasons = append(reasons, "UNASSIGNED_SOURCE_TEXT")
	}
	if b.Paragraph != nil {
		for _, r := range b.Paragraph.Runs {
			reasons = appendReasons(reasons, r.ReviewReasons...)
		}
		if n := b.Paragraph.Numbering; n != nil && !n.Resolved {
			reasons = appendReasons(reasons, "NUMBERING_REQUIRES_REVIEW")
		}
	}
	return reasons
}

func contextReasons(locations []Locator, contexts map[Locator]string) []string {
	var reasons []string
	for _, loc := range locations {
		reasons = appendReasons(reasons, blockReasons(SourceBlock{Kind: contexts[loc], PartKind: "document"})...)
	}
	return reasons
}

func changedProperties(props []Property) bool {
	for _, p := range props {
		if wordName(p.Name, "rPrChange") || wordName(p.Name, "pPrChange") || wordName(p.Name, "numPrChange") || changedProperties(p.Children) {
			return true
		}
	}
	return false
}

func appendReasons(existing []string, reasons ...string) []string {
	for _, reason := range reasons {
		if !slices.Contains(existing, reason) {
			existing = append(existing, reason)
		}
	}
	return existing
}

func complexFieldRanges(blocks []SourceBlock) []SourceRange {
	depth, start := 0, 0
	intervals := []SourceRange{}
	for _, b := range blocks {
		if b.Object == nil || !wordName(b.Object.Name, "fldChar") {
			continue
		}
		switch attr(*b.Object, "fldCharType") {
		case "begin":
			if depth == 0 {
				start = b.Order
			}
			depth++
		case "end":
			depth = max(0, depth-1)
			if depth == 0 {
				intervals = append(intervals, SourceRange{Order: start, End: b.End})
			}
		}
	}
	if depth > 0 {
		intervals = append(intervals, SourceRange{Order: start, End: int(^uint(0) >> 1)})
	}
	return intervals
}

func markComplexFields(blocks []SourceBlock) {
	intervals := complexFieldRanges(blocks)
	index := 0
	for i := range blocks {
		b := &blocks[i]
		if b.Paragraph == nil {
			continue
		}
		for index < len(intervals) && intervals[index].End < b.Order {
			index++
		}
		if index < len(intervals) && intervals[index].Order <= b.End {
			b.ReviewReasons = appendReasons(b.ReviewReasons, fieldReview)
		}
	}
}
