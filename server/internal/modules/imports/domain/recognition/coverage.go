package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"unicode"
	"unicode/utf8"
)

func (r *recognizer) checkCoverage(docs []domain.EvidenceDocument) {
	for _, d := range docs {
		for _, b := range d.Blocks {
			if !b.Meaningful {
				continue
			}
			entry := r.out.Coverage[r.coverage[d.SourceID+"/"+b.ID]]
			if !covered(b, entry.Uses) {
				r.issue("UNASSIGNED_SOURCE_BLOCK", blocking, b.ID, "source", []domain.SourceRef{{SourceID: d.SourceID, BlockID: b.ID, Start: 0, End: utf8.RuneCountInString(b.Text)}})
			}
		}
	}
}

func covered(b domain.EvidenceBlock, uses []domain.CoverageUse) bool {
	if len(uses) == 0 {
		return false
	}
	ordered := slices.Clone(uses)
	slices.SortFunc(ordered, func(a, b domain.CoverageUse) int { return a.Start - b.Start })
	text := []rune(b.Text)
	position := 0
	generated := b.Numbering == ""
	for _, use := range ordered {
		if use.GeneratedLabel {
			generated = true
		}
		for _, ch := range text[position:max(position, use.Start)] {
			if !unicode.IsSpace(ch) {
				return false
			}
		}
		position = max(position, use.End)
	}
	for _, ch := range text[position:] {
		if !unicode.IsSpace(ch) {
			return false
		}
	}
	return generated
}
