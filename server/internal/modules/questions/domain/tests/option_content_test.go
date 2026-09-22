package domain_test

import (
	"encoding/json"
	"quizzivy/internal/modules/questions/domain"
	"testing"
)

func TestOptionWritesRequireMatchingProjection(t *testing.T) {
	raw := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"word","marks":["underline"]}]}]}`)
	in := domain.Input{Type: domain.SingleChoice, Prompt: "Pronunciation", Points: "1.00", Options: []domain.OptionInput{
		{Text: "word", Content: raw, IsCorrect: true}, {Text: "other"},
	}}
	if err := in.Validate(nil); err != nil {
		t.Fatal(err)
	}
	in.Options[0].Text = "mismatch"
	if err := in.Validate(nil); err == nil {
		t.Fatal("accepted an option whose displayed content differs from its text")
	}
	in.Options[0].Content = json.RawMessage(`null`)
	if err := in.Validate(nil); err != nil {
		t.Fatalf("explicit formatting removal: %v", err)
	}
}
