package domain_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/content"

	"golang.org/x/text/unicode/norm"
)

func nfd(s string) string { return norm.NFD.String(s) }

func nfc(s string) string { return norm.NFC.String(s) }

func prose(t testing.TB, text string) (json.RawMessage, string) {
	t.Helper()
	raw := fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]},{"type":"gap","id":"Gap-1","label":%q}]}]}`, text, nfd("Điền"))
	document, err := content.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(raw), document.PlainText()
}

func optionProse(t testing.TB, text string) (json.RawMessage, string) {
	t.Helper()
	raw := fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}]}`, text)
	document, err := content.ParseOption([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(raw), document.PlainText()
}

func decomposedQuestion(t testing.TB) domain.Input {
	t.Helper()
	promptRaw, promptText := prose(t, nfd("Nghe và điền: "))
	explanationRaw, explanationText := prose(t, nfd("Vì sao: "))
	optionRaw, optionText := optionProse(t, nfd("Phương án một"))
	alt, answer, transcript, tag := nfd("Ảnh cái nón"), nfd("Hà Nội"), nfd("Lời thoại"), nfd("nghé")
	gap, blankID, assetID := "Gap-1", "blank-id", "01935000-0000-7000-8000-0000000000a1"
	explanation, sample := explanationText, nfd("Mẫu trả lời")
	return domain.Input{
		Type:               domain.FillBlank,
		Prompt:             promptText,
		PromptContent:      promptRaw,
		Explanation:        &explanation,
		ExplanationContent: explanationRaw,
		SampleAnswer:       &sample,
		Transcript:         &transcript,
		MediaAlt:           &alt,
		MediaAssetID:       &assetID,
		Tags:               []string{tag, "shared"},
		Options:            []domain.OptionInput{{Content: optionRaw, Text: optionText, ID: &blankID}},
		Blanks:             []domain.BlankInput{{GapID: &gap, ID: &blankID, Ordinal: 1, AcceptedAnswers: []string{answer, "Hanoi"}}},
	}
}

func TestComposedReturnsEveryTextInTheInputInNFC(t *testing.T) {
	in := decomposedQuestion(t)
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"prompt":         got.Prompt,
		"promptContent":  string(got.PromptContent),
		"explanation":    *got.Explanation,
		"explContent":    string(got.ExplanationContent),
		"sampleAnswer":   *got.SampleAnswer,
		"transcript":     *got.Transcript,
		"mediaAlt":       *got.MediaAlt,
		"tag":            got.Tags[0],
		"optionText":     got.Options[0].Text,
		"optionContent":  string(got.Options[0].Content),
		"acceptedAnswer": got.Blanks[0].AcceptedAnswers[0],
	} {
		if value != nfc(value) {
			t.Errorf("%s is still decomposed: %q", name, value)
		}
		if strings.TrimSpace(value) == "" {
			t.Errorf("%s was emptied", name)
		}
	}
	if got.Tags[1] != "shared" || got.Blanks[0].AcceptedAnswers[1] != "Hanoi" {
		t.Fatalf("composed ASCII changed: %v %v", got.Tags, got.Blanks[0].AcceptedAnswers)
	}
}

func TestComposedKeepsTheCompanionTextEqualToTheDocument(t *testing.T) {
	got, err := decomposedQuestion(t).Composed()
	if err != nil {
		t.Fatal(err)
	}
	document, err := content.ParseQuestionPrompt(got.PromptContent)
	if err != nil {
		t.Fatal(err)
	}
	if document.PlainText() != got.Prompt {
		t.Fatalf("prompt %q is not the projection %q", got.Prompt, document.PlainText())
	}
	if err := got.Options[0].ValidateContent(); err != nil {
		t.Fatalf("option text no longer matches its document: %v", err)
	}
}

func TestComposedKeepsAMarkAtANodeBoundaryAsItsDocumentHasIt(t *testing.T) {
	raw := json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"e","marks":["bold"]},{"type":"text","text":"́n","marks":[]}]}]}`)
	document, err := content.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	in := domain.Input{Type: domain.ShortAnswer, Prompt: document.PlainText(), PromptContent: raw, Points: "1"}
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	if err := got.ValidateContent(); err != nil {
		t.Fatalf("the pair stopped matching: %v", err)
	}
}

func TestComposedLeavesIdentitiesAndMissingFieldsAlone(t *testing.T) {
	in := decomposedQuestion(t)
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	if *got.MediaAssetID != *in.MediaAssetID || *got.Blanks[0].GapID != "Gap-1" || *got.Blanks[0].ID != "blank-id" || *got.Options[0].ID != "blank-id" {
		t.Fatalf("an identity changed: %v %v", got.Blanks, got.Options)
	}
	bare, err := (domain.Input{Type: domain.ShortAnswer, Prompt: nfd("Ví dụ"), Points: "1"}).Composed()
	if err != nil {
		t.Fatal(err)
	}
	if bare.Explanation != nil || bare.SampleAnswer != nil || bare.Transcript != nil || bare.MediaAlt != nil || bare.Tags != nil || bare.Options != nil || bare.Blanks != nil || bare.PromptContent != nil {
		t.Fatalf("an absent field appeared: %+v", bare)
	}
	if bare.Prompt != nfc(nfd("Ví dụ")) {
		t.Fatalf("prompt=%q", bare.Prompt)
	}
}

func TestComposedDoesNotMutateTheInputItWasGiven(t *testing.T) {
	in := decomposedQuestion(t)
	before := in.Options[0].Text
	tag := in.Tags[0]
	answer := in.Blanks[0].AcceptedAnswers[0]
	if _, err := in.Composed(); err != nil {
		t.Fatal(err)
	}
	if in.Options[0].Text != before || in.Tags[0] != tag || in.Blanks[0].AcceptedAnswers[0] != answer {
		t.Fatal("Composed rewrote the slices of its receiver")
	}
}

func TestComposedRefusesAnAltTextComposingLeavesOverItsLimit(t *testing.T) {
	alt := strings.Repeat("क़", domain.MaxMediaAltLength)
	_, err := (domain.Input{Type: domain.ShortAnswer, Prompt: "Mô tả", Points: "1", MediaAlt: &alt}).Composed()
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "mediaAlt" {
		t.Fatalf("err=%v", err)
	}
}

func TestComposedRefusesADocumentComposingMakesInvalid(t *testing.T) {
	raw := json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"g1","label":%q}]}]}`, strings.Repeat("क़", 32)))
	document, err := content.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	in := domain.Input{Type: domain.FillBlank, Prompt: document.PlainText(), PromptContent: raw, Points: "1"}
	_, err = in.Composed()
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "promptContent" {
		t.Fatalf("err=%v", err)
	}
}

func TestComposedLeavesAnInvalidDocumentToValidation(t *testing.T) {
	in := domain.Input{Type: domain.ShortAnswer, Prompt: nfd("Ví dụ"), PromptContent: json.RawMessage(`{"format":"nope"}`), Points: "1"}
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	if string(got.PromptContent) != `{"format":"nope"}` {
		t.Fatalf("an invalid document was rewritten: %s", got.PromptContent)
	}
	if err := got.ValidateContent(); err == nil {
		t.Fatal("the invalid document was accepted")
	}
}
