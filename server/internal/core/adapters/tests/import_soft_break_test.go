package adapters_test

import (
	"bytes"
	"context"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/platform/word"
	"testing"
)

// QC-R6IMP-2: a Shift+Enter break (<w:br/>) inside an instruction paragraph does not cut the instruction in two.
func TestASoftBreakInAWordInstructionKeepsTheInstructionWhole(t *testing.T) {
	data := evidenceWord(t, `<w:p><w:r><w:t xml:space="preserve">I. Answer the questions. Write NO MORE THAN</w:t><w:br/><w:t>TWO WORDS for each answer.</w:t></w:r></w:p>`+
		`<w:p><w:r><w:t>1. Where does Anna work?</w:t></w:r></w:p>`+
		`<w:p><w:r><w:t xml:space="preserve">II. Write TRUE, FALSE or</w:t><w:br/><w:t>NOT GIVEN.</w:t></w:r></w:p>`+
		`<w:p><w:r><w:t>2. The shop opens on Sunday.</w:t></w:r></w:p>`)
	raw, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "synthetic", word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	d, err := recognition.Recognize(context.Background(), []domain.EvidenceDocument{adapters.ImportEvidence(raw, "exam")}, domain.RecognitionProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Sections) != 2 || d.Sections[0].Title != "I. Answer the questions. Write NO MORE THAN TWO WORDS for each answer." || d.Sections[1].Title != "II. Write TRUE, FALSE or NOT GIVEN." {
		t.Fatalf("each instruction is one title: %+v", d.Sections)
	}
	short, tfng := d.Sections[0].Items[0].Question, d.Sections[1].Items[0].Question
	if short == nil || short.MaxWords == nil || *short.MaxWords != 2 {
		t.Fatalf("word limit across the break: %+v", d.Sections[0].Items[0])
	}
	if tfng == nil || tfng.Type != "tfng" {
		t.Fatalf("tfng across the break: %+v", d.Sections[1].Items[0])
	}
}
