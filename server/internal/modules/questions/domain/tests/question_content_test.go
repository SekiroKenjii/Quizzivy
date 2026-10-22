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
		t.Fatal("accepted rich fill-blank without gap bindings")
	}
	in.PromptContent = json.RawMessage(`null`)
	if err := in.ValidateContent(); err != nil {
		t.Fatal(err)
	}
}

// TestMediaInProseIsValidContent pins R6-13's half of D-064: the bank's
// content check accepts DG-109's image and audio blocks in a prompt and an
// explanation (ownership is the store's check, under the asset locks), and a
// prompt's audio marker needs the question's own audio to place.
func TestMediaInProseIsValidContent(t *testing.T) {
	text := "Look."
	image := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]},{"type":"image","assetId":"01935000-0000-7000-8000-00000000000a","alt":""}]}`)
	marker := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"audio"},{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]}]}`)
	audio := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]},{"type":"audio","assetId":"01935000-0000-7000-8000-00000000000b"}]}`)
	for name, in := range map[string]domain.Input{
		"an image in the prompt":      {Type: domain.ShortAnswer, Prompt: text, PromptContent: image, Points: "1"},
		"the audio marker":            {Type: domain.ShortAnswer, Prompt: text, PromptContent: marker, Points: "1"},
		"an audio in the explanation": {Type: domain.ShortAnswer, Prompt: text, Explanation: &text, ExplanationContent: audio, Points: "1"},
		"an image in the explanation": {Type: domain.ShortAnswer, Prompt: text, Explanation: &text, ExplanationContent: image, Points: "1"},
	} {
		if err := in.ValidateContent(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	audioID, audioKind, imageKind := "01935000-0000-7000-8000-00000000000c", "audio", "image"
	policy := &domain.AudioPolicy{AllowSeek: true}
	placed := domain.Input{Type: domain.ShortAnswer, Prompt: text, PromptContent: marker, Points: "1"}
	for name, tc := range map[string]struct {
		asset, kind *string
		audio       *domain.AudioPolicy
		ok          bool
	}{
		"without question media":  {},
		"with an image":           {asset: &audioID, kind: &imageKind},
		"with the question audio": {asset: &audioID, kind: &audioKind, audio: policy, ok: true},
	} {
		in := placed
		in.MediaAssetID, in.Audio = tc.asset, tc.audio
		err := in.Validate(tc.kind)
		switch {
		case tc.ok && err != nil:
			t.Errorf("the marker %s: %v", name, err)
		case !tc.ok && (err == nil || !hasField(t, err, "promptContent")):
			t.Errorf("the marker %s: %v, want a promptContent refusal", name, err)
		}
	}
	if err := (domain.Input{Type: domain.ShortAnswer, Prompt: text, PromptContent: image, Points: "1"}).ValidatePlacedAudio(nil); err != nil {
		t.Errorf("a prompt without the marker needs no audio: %v", err)
	}
}
