package recognition_test

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/shared/content"
	"strings"
	"testing"
	"unicode/utf8"
)

type part struct {
	text    string
	marks   []string
	colored bool
}

func plain(text string) part { return part{text: text} }

func underlined(text string) part { return part{text: text, marks: []string{"underline"}} }

func red(text string) part { return part{text: text, colored: true} }

type block struct {
	parts  []part
	table  string
	row    int
	column int
}

func p(parts ...part) block { return block{parts: parts} }

func lines(texts ...string) []block {
	out := make([]block, len(texts))
	for i, t := range texts {
		out[i] = p(plain(t))
	}
	return out
}

func row(table string, index int, cells ...string) []block {
	out := make([]block, len(cells))
	for i, c := range cells {
		out[i] = block{parts: []part{plain(c)}, table: table, row: index, column: i}
	}
	return out
}

func document(role string, blocks ...[]block) domain.EvidenceDocument {
	d := domain.EvidenceDocument{SourceID: role + "-source", Role: role, Version: "ooxml-blocks-v1", Findings: []domain.EvidenceFinding{}}
	n := 0
	for _, group := range blocks {
		for _, b := range group {
			n++
			var text strings.Builder
			var spans []domain.EvidenceSpan
			for _, part := range b.parts {
				start := utf8.RuneCountInString(text.String())
				text.WriteString(part.text)
				marks := part.marks
				if marks == nil {
					marks = []string{}
				}
				spans = append(spans, domain.EvidenceSpan{Start: start, End: start + utf8.RuneCountInString(part.text), Marks: marks, Colored: part.colored})
			}
			d.Blocks = append(d.Blocks, domain.EvidenceBlock{ID: fmt.Sprintf("%s-%d", role, n), Kind: "paragraph", Main: true, Safe: true, Meaningful: true, Text: text.String(), Spans: spans, Reasons: []string{}, TableID: b.table, Row: b.row, Column: b.column})
		}
	}
	return d
}

func exam(blocks ...[]block) domain.EvidenceDocument { return document("exam", blocks...) }

func key(blocks ...[]block) domain.EvidenceDocument { return document("answer_key", blocks...) }

func recognize(t *testing.T, docs ...domain.EvidenceDocument) domain.Draft {
	t.Helper()
	return recognizeWith(t, domain.RecognitionProfile{}, docs...)
}

func recognizeWith(t *testing.T, profile domain.RecognitionProfile, docs ...domain.EvidenceDocument) domain.Draft {
	t.Helper()
	d, err := recognition.Recognize(context.Background(), docs, profile)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func question(t *testing.T, d domain.Draft, label string) *domain.DraftQuestion {
	t.Helper()
	for _, q := range d.Questions() {
		if q.Label == label {
			return q
		}
	}
	t.Fatalf("question %s not found among %v", label, labels(d))
	return nil
}

func labels(d domain.Draft) []string {
	var out []string
	for _, q := range d.Questions() {
		out = append(out, q.Label)
	}
	return out
}

func text(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	doc, err := content.Parse(raw)
	if err != nil {
		t.Fatalf("invalid content %s: %v", raw, err)
	}
	return doc.PlainText()
}

func answerLabels(q *domain.DraftQuestion) []string {
	var out []string
	for _, id := range q.Answer.OptionIDs {
		for _, o := range q.Options {
			if o.ID == id {
				out = append(out, o.Label)
			}
		}
	}
	return out
}

func notices(d domain.Draft, code string) []domain.Finding {
	var out []domain.Finding
	for _, n := range d.Notices {
		if n.Code == code {
			out = append(out, n)
		}
	}
	return out
}

func groups(d domain.Draft) []*domain.DraftGroup {
	var out []*domain.DraftGroup
	for s := range d.Sections {
		for i := range d.Sections[s].Items {
			if g := d.Sections[s].Items[i].Group; g != nil {
				out = append(out, g)
			}
		}
	}
	return out
}

func recognizeErr(docs ...domain.EvidenceDocument) (domain.Draft, error) {
	return recognition.Recognize(context.Background(), docs, domain.RecognitionProfile{})
}
