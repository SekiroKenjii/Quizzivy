package domain_test

import (
	"encoding/json"
	"quizzivy/internal/modules/questions/domain"
	"testing"
)

func TestQuestionProseValidationAppliesToDirectInputs(t *testing.T) {
	text := "Tiếng Việt"
	doc := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Tiếng Việt","marks":["underline"]}]}]}`)
	in := domain.Input{Type: domain.ShortAnswer, Prompt: text, PromptContent: doc, Explanation: &text, ExplanationContent: doc, Points: "1"}
	if err := in.Validate(nil); err != nil {
		t.Fatal(err)
	}
	in.Prompt = "changed"
	if in.Validate(nil) == nil {
		t.Fatal("accepted mismatched prompt")
	}
	in.Prompt = text
	in.Explanation = nil
	if in.Validate(nil) == nil {
		t.Fatal("accepted explanation without its projection")
	}
	in.Explanation = &text
	in.Type = domain.FillBlank
	if in.ValidateContent() == nil {
		t.Fatal("enabled rich fill-blank before gap binding")
	}
	in.PromptContent = json.RawMessage(`null`)
	if err := in.ValidateContent(); err != nil {
		t.Fatal(err)
	}
}
