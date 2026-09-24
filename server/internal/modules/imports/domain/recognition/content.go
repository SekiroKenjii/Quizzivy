// Package recognition proposes source-linked exam structure without generating answers or writing assessment entities.
package recognition

import (
	"encoding/json"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/content"
	"slices"
	"unicode"
)

const inlineText = "text"

type inline struct {
	Type  string   `json:"type"`
	Text  string   `json:"text"`
	Marks []string `json:"marks"`
}
type paragraph struct {
	Type    string   `json:"type"`
	Content []inline `json:"content"`
}
type document struct {
	Format string      `json:"format"`
	Blocks []paragraph `json:"blocks"`
}

func prose(b domain.EvidenceBlock, start, end int, removeMark string) (json.RawMessage, error) {
	runes := []rune(b.Text)
	if start < 0 || end < start || end > len(runes) {
		return nil, domain.ErrInvalid
	}
	parts := []inline{}
	position := start
	for _, span := range b.Spans {
		a, z := max(start, span.Start), min(end, span.End)
		if a >= z {
			continue
		}
		if a > position {
			parts = append(parts, inline{Type: inlineText, Text: string(runes[position:a]), Marks: []string{}})
		}
		marks := slices.DeleteFunc(slices.Clone(span.Marks), func(m string) bool { return m == removeMark })
		if marks == nil {
			marks = []string{}
		}
		parts = append(parts, inline{Type: inlineText, Text: string(runes[a:z]), Marks: marks})
		position = z
	}
	if position < end {
		parts = append(parts, inline{Type: inlineText, Text: string(runes[position:end]), Marks: []string{}})
	}
	raw, err := json.Marshal(document{Format: "semantic_v1", Blocks: []paragraph{{Type: "paragraph", Content: parts}}})
	if err != nil {
		return nil, err
	}
	if _, err := content.ParseQuestion(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func marked(b domain.EvidenceBlock, start, end int, mark string) bool {
	if mark == "" {
		return false
	}
	runes := []rune(b.Text)
	found := false
	position := start
	for _, span := range b.Spans {
		a, z := max(start, span.Start), min(end, span.End)
		if a >= z {
			continue
		}
		if visible(runes[position:a]) {
			return false
		}
		if visible(runes[a:z]) {
			if !slices.Contains(span.Marks, mark) {
				return false
			}
			found = true
		}

		position = z
	}
	if visible(runes[position:end]) {
		return false
	}
	return found
}

func trimRange(text string, start, end int) (int, int) {
	runes := []rune(text)
	for start < end && unicode.IsSpace(runes[start]) {
		start++
	}
	for end > start && unicode.IsSpace(runes[end-1]) {
		end--
	}
	return start, end
}

func visible(chars []rune) bool {
	for _, r := range chars {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func (r *recognizer) continuation(b domain.EvidenceBlock, start, end int) error {
	a, z := trimRange(b.Text, start, end)
	raw, err := prose(b, a, z, "")
	if err != nil {
		return err
	}
	q := &r.out.Questions[r.question]
	var before, addition document
	if err := json.Unmarshal(q.Prompt, &before); err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &addition); err != nil {
		return err
	}
	before.Blocks = append(before.Blocks, addition.Blocks...)
	combined, err := json.Marshal(before)
	if err != nil {
		return err
	}
	if _, err := content.ParseQuestion(combined); err != nil {
		return err
	}
	q.Prompt = combined
	refs := []domain.SourceRef{r.ref(b, a, z)}
	q.Fields = append(q.Fields, domain.FieldEvidence{Field: "prompt", Origin: inferredStructure, Refs: refs})
	r.use(b, start, end, q.ID, "prompt", false)
	r.issue("CONTEXT_ATTACHMENT_INFERRED", reviewRequired, q.ID, "prompt", refs)
	return nil
}
