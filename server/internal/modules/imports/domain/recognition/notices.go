package recognition

import (
	"maps"
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
	"PDF_MARKS_UNAVAILABLE":             true,
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

type sourceTally struct {
	count int
	main  bool
	refs  []domain.SourceRef
}

func tallySource(d domain.EvidenceDocument) map[string]*sourceTally {
	tallies := map[string]*sourceTally{}
	tally := func(code string, main bool) *sourceTally {
		t, ok := tallies[code]
		if !ok {
			t = &sourceTally{}
			tallies[code] = t
		}
		t.count++
		t.main = t.main || main
		return t
	}
	for _, f := range d.Findings {
		tally(f.Code, f.Main)
	}
	for _, block := range d.Blocks {
		if !block.Main && !block.Meaningful {
			continue
		}
		for _, reason := range block.Reasons {
			if reason != inlineObject {
				t := tally(reason, block.Main)
				t.refs = append(t.refs, domain.SourceRef{SourceID: d.SourceID, BlockID: block.ID, End: len([]rune(block.Text))})
			}
		}
	}
	return tallies
}

func (b *builder) sourceNotices(docs []domain.EvidenceDocument) {
	for _, d := range docs {
		tallies := tallySource(d)
		for _, code := range slices.Sorted(maps.Keys(tallies)) {
			t := tallies[code]
			severity := domain.ReviewRequired
			if informationalSource[code] || !t.main {
				severity = domain.Informational
			}
			b.notice(domain.CodeSourceObject, severity, d.SourceID, code, t.count, t.refs)
		}
	}
}
