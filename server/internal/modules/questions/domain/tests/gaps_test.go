package domain_test

import (
	"encoding/json"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/content"
	"testing"
)

func gapInput() domain.Input {
	a, b := "a", "b"
	return domain.Input{Type: domain.FillBlank, Prompt: "[2]\t[1]", Points: "1",
		PromptContent: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"table","rows":[[{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"gap","id":"b","label":"2"}]}]},{"header":false,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"gap","id":"a","label":"1"}]}]}]]}]}`),
		Blanks:        []domain.BlankInput{{GapID: &a, Ordinal: 1, AcceptedAnswers: []string{"one"}}, {GapID: &b, Ordinal: 2, AcceptedAnswers: []string{"two"}}},
	}
}

func TestRichBlankBindingsAreExactAndIndependentOfOrder(t *testing.T) {
	in := gapInput()
	if err := in.Validate(nil); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*domain.Input){
		func(in *domain.Input) { in.Blanks = in.Blanks[:1] },
		func(in *domain.Input) { in.Blanks[0].GapID = nil },
		func(in *domain.Input) { in.Blanks[0].GapID = in.Blanks[1].GapID },
		func(in *domain.Input) { in.Blanks[0].Ordinal = 32768 },
		func(in *domain.Input) { id := "missing"; in.Blanks[0].GapID = &id },
		func(in *domain.Input) { in.Type = domain.ShortAnswer; in.Blanks = nil },
		func(in *domain.Input) { in.ExplanationContent = in.PromptContent; in.Explanation = &in.Prompt },
		func(in *domain.Input) { in.PromptContent = nil; in.Prompt = "{{1}} {{2}}" },
	} {
		invalid := gapInput()
		mutate(&invalid)
		if invalid.Validate(nil) == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}

func TestRichBlankCopiesRejectCollidingIdentitiesBeforeMutation(t *testing.T) {
	in := gapInput()
	original := string(in.PromptContent)
	if err := in.RebindGaps(func() string { return "collision" }); err == nil {
		t.Fatal("accepted duplicate target identities")
	}
	if string(in.PromptContent) != original || *in.Blanks[0].GapID != "a" || *in.Blanks[1].GapID != "b" {
		t.Fatal("failed rebind modified the source input")
	}
}

func TestRichBlankCopiesRemapBothEndsWithoutChangingAnswers(t *testing.T) {
	in := gapInput()
	source := string(in.PromptContent)
	ids := []string{"new-b", "new-a"}
	n := 0
	if err := in.RebindGaps(func() string { id := ids[n]; n++; return id }); err != nil {
		t.Fatal(err)
	}
	if err := in.Validate(nil); err != nil {
		t.Fatal(err)
	}
	doc, err := content.ParseQuestionPrompt(in.PromptContent)
	if err != nil || doc.PlainText() != "[2]\t[1]" || *in.Blanks[0].GapID != "new-a" || in.Blanks[0].AcceptedAnswers[0] != "one" {
		t.Fatal("copy changed presentation or answer attachment")
	}
	if source != string(gapInput().PromptContent) || string(in.PromptContent) == source {
		t.Fatal("copy failed to isolate prompt identities")
	}
}
