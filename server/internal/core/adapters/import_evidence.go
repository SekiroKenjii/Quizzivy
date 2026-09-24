package adapters

import (
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/word"
	"slices"
	"strings"
)

// ImportEvidence projects safe recognition text without losing private source offsets or raw-evidence findings.
func ImportEvidence(raw word.Extraction, role string) domain.EvidenceDocument {
	out := domain.EvidenceDocument{SourceID: raw.SourceID, Role: role, Version: raw.Version, Blocks: make([]domain.EvidenceBlock, 0, len(raw.Blocks))}
	out.Findings = []string{}
	for _, f := range raw.Findings {
		if !slices.Contains(out.Findings, f.Code) {
			out.Findings = append(out.Findings, f.Code)
		}
	}
	cells := make(map[string]*word.CellEvidence)
	parents := make(map[string]string)
	for _, b := range raw.Blocks {
		if privateColor(b.Properties) && !slices.Contains(out.Findings, "SOURCE_COLOR_REQUIRES_REVIEW") {
			out.Findings = append(out.Findings, "SOURCE_COLOR_REQUIRES_REVIEW")
		}
		parents[b.ID] = b.ParentID
		if b.Cell != nil {
			cells[b.ID] = b.Cell
		}
	}
	for _, b := range raw.Blocks {
		v := projectEvidenceBlock(b, raw.MainPart)
		v = projectCell(v, b.ParentID, parents, cells)
		out.Blocks = append(out.Blocks, v)
	}
	return out
}

func projectEvidenceBlock(b word.SourceBlock, main string) domain.EvidenceBlock {
	v := domain.EvidenceBlock{ID: b.ID, Kind: b.Kind, Main: b.Part == main, Meaningful: b.Meaningful, Safe: b.Kind == "paragraph" && b.Part == main, Spans: []domain.EvidenceSpan{}, Reasons: slices.Clone(b.ReviewReasons)}
	if b.Paragraph == nil {
		return v
	}
	var text strings.Builder
	if n := b.Paragraph.Numbering; n != nil && n.Resolved {
		v.Numbering = n.Text
	}
	for _, run := range b.Paragraph.Runs {
		marks, loss := projectMarks(run.Marks)
		loss = loss || privateColor(run.Properties)
		if loss && !slices.Contains(v.Reasons, "SEMANTIC_FORMATTING_LOSS") {
			v.Reasons = append(v.Reasons, "SEMANTIC_FORMATTING_LOSS")
		}
		for _, f := range run.Fragments {
			text.WriteString(f.Text)
			v.Spans = append(v.Spans, domain.EvidenceSpan{Start: f.Start, End: f.EndOffset, Marks: marks})
		}
	}
	v.Text = text.String()
	for _, reason := range v.Reasons {
		switch reason {
		case "HIDDEN_TEXT_REQUIRES_REVIEW", "TRACKED_CHANGE_REQUIRES_REVIEW", "FIELD_REQUIRES_REVIEW", "TEXTBOX_ORDER_REQUIRES_REVIEW", "ANCILLARY_CONTENT_REQUIRES_REVIEW":
			v.Safe = false
		}
	}
	return v
}

func projectMarks(marks []word.ResolvedMark) ([]string, bool) {
	out := []string{}
	loss := false
	for _, mark := range marks {
		value, incomplete := projectMark(mark)
		loss = loss || incomplete
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out, loss
}

const strikeMark = "strike"

func projectMark(mark word.ResolvedMark) (string, bool) {
	if !mark.Resolved {
		return "", true
	}
	simple := map[string]string{"b": "bold", "i": "italic", strikeMark: strikeMark}
	if value, ok := simple[mark.Name]; ok {
		if mark.Value == "on" {
			return value, false
		}
		return "", false
	}
	switch mark.Name {
	case "u":
		if mark.Value != "none" && mark.Value != "off" {
			return "underline", mark.Value != "single"
		}
	case "dstrike":
		if mark.Value == "on" {
			return strikeMark, true
		}
	case "caps", "smallCaps":
		return "", mark.Value == "on"
	case "vertAlign":
		if mark.Value == "subscript" || mark.Value == "superscript" {
			return mark.Value, false
		}
	case "color":
		return "", mark.Value != "auto" && !strings.EqualFold(mark.Value, "000000")
	case "highlight":
		return "", mark.Value != "none"
	}
	return "", false
}

func privateColor(props []word.Property) bool {
	for _, p := range props {
		if strings.HasSuffix(p.Name, "}color") || strings.HasSuffix(p.Name, "}highlight") || strings.HasSuffix(p.Name, "}shd") {
			return true
		}
		if privateColor(p.Children) {
			return true
		}
	}
	return false
}

func projectCell(v domain.EvidenceBlock, parent string, parents map[string]string, cells map[string]*word.CellEvidence) domain.EvidenceBlock {
	for id, depth := parent, 0; id != "" && depth < 128; id, depth = parents[id], depth+1 {
		if cell := cells[id]; cell != nil {
			v.TableID = cell.TableID
			v.Row = cell.Row
			v.Column = cell.Column
			if !cell.Resolved {
				v.Reasons = append(v.Reasons, "TABLE_GRID_REQUIRES_REVIEW")
			}
			break
		}
	}
	return v
}
