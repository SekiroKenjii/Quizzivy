package recognition

import (
	"quizzivy/internal/modules/imports/domain"
	"slices"
)

const maxEvidence = 50

var informationalSource = map[string]bool{
	"SEMANTIC_MARK_REQUIRES_REVIEW":     true,
	"STYLE_REQUIRES_REVIEW":             true,
	"STYLE_RESOLUTION_REQUIRED":         true,
	"TABLE_STYLE_REQUIRES_REVIEW":       true,
	"NUMBERING_RESOLUTION_REQUIRED":     true,
	"COLUMN_ORDER_REQUIRES_REVIEW":      true,
	"SEMANTIC_FORMATTING_LOSS":          true,
	"SOURCE_COLOR_REQUIRES_REVIEW":      true,
	"TABLE_GRID_REQUIRES_REVIEW":        true,
	"ANCILLARY_CONTENT_REQUIRES_REVIEW": true,
}

func (b *builder) notice(code string, severity domain.Severity, target, field string, count int, evidence []domain.SourceRef) {
	if len(evidence) > maxEvidence {
		evidence = evidence[:maxEvidence]
	}
	first := ""
	if len(evidence) > 0 {
		first = evidence[0].SourceID + "/" + evidence[0].BlockID
	}
	id := identity("finding", code, target, field, first)
	for i := range b.notices {
		if b.notices[i].ID == id {
			b.notices[i].Count += count
			return
		}
	}
	b.notices = append(b.notices, domain.Finding{ID: id, Code: code, Severity: severity, Target: target, Field: field, Count: count, Evidence: nonNil(slices.Clone(evidence))})
}

type unassignedRun struct {
	refs  []domain.SourceRef
	lines int
}

func (b *builder) unassignedRuns(lines []*line) {
	var run unassignedRun
	flush := func() {
		if run.lines > 0 {
			b.notice(domain.CodeUnassignedText, domain.ReviewRequired, "", "", run.lines, run.refs)
		}
		run = unassignedRun{}
	}
	for _, l := range lines {
		ranges := l.unused()
		if len(ranges) == 0 {
			flush()
			continue
		}
		for _, r := range ranges {
			run.refs = append(run.refs, l.refs(r[0], r[1])...)
		}
		run.lines++
	}
	flush()
}

func (b *builder) unassigned(l *line) { b.unassignedRuns([]*line{l}) }

func (b *builder) extraPapers() {
	lines := b.exam.extraPaper
	if len(lines) == 0 {
		return
	}
	b.notice(domain.CodeMultiplePapers, domain.ReviewRequired, "", "", len(lines), lines[0].refs(0, len(lines[0].text)))
}

func (b *builder) sourceNotices(docs []domain.EvidenceDocument) {
	for _, d := range docs {
		counts := map[string]int{}
		refs := map[string][]domain.SourceRef{}
		main := map[string]bool{}
		for _, f := range d.Findings {
			counts[f.Code]++
			main[f.Code] = main[f.Code] || f.Main
		}
		for _, block := range d.Blocks {
			for _, reason := range block.Reasons {
				if !block.Main && !block.Meaningful {
					continue
				}
				counts[reason]++
				main[reason] = main[reason] || block.Main
				refs[reason] = append(refs[reason], domain.SourceRef{SourceID: d.SourceID, BlockID: block.ID, End: len([]rune(block.Text))})
			}
		}
		codes := make([]string, 0, len(counts))
		for code := range counts {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		for _, code := range codes {
			severity := domain.ReviewRequired
			if informationalSource[code] || !main[code] {
				severity = domain.Informational
			}
			b.notice(domain.CodeSourceObject, severity, d.SourceID, code, counts[code], refs[code])
		}
	}
}
