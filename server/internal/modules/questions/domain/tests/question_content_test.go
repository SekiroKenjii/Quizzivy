package domain_test

import (
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/questions/domain"
	"strings"
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
		t.Fatal("accepted rich fill-blank without gap bindings")
	}
	in.PromptContent = json.RawMessage(`null`)
	if err := in.ValidateContent(); err != nil {
		t.Fatal(err)
	}
}

// TestMediaInProseIsRefusedUntilTheBankStoresIt pins D-064's ruling 7: the
// contract describes DG-109's image and audio blocks, and every bank write
// refuses them with a field error naming the field, valid as they are, until
// R6-13 validates ownership, the question's audio and the freeze.
func TestMediaInProseIsRefusedUntilTheBankStoresIt(t *testing.T) {
	text := "Look."
	image := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]},{"type":"image","assetId":"01935000-0000-7000-8000-00000000000a","alt":""}]}`)
	marker := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"audio"},{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]}]}`)
	audio := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]},{"type":"audio","assetId":"01935000-0000-7000-8000-00000000000b"}]}`)
	for name, in := range map[string]domain.Input{
		"promptContent":      {Type: domain.ShortAnswer, Prompt: text, PromptContent: image, Points: "1"},
		"promptContent ":     {Type: domain.ShortAnswer, Prompt: text, PromptContent: marker, Points: "1"},
		"explanationContent": {Type: domain.ShortAnswer, Prompt: text, Explanation: &text, ExplanationContent: audio, Points: "1"},
	} {
		var invalid *domain.ValidationError
		err := in.ValidateContent()
		if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != strings.TrimSpace(name) || !strings.Contains(invalid.Fields[0].Message, "hình ảnh hoặc âm thanh") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
