package domain

import (
	"bytes"
	"encoding/json"
	"quizzivy/internal/shared/content"
)

// ValidateContent checks the prose profiles and exact companion text before writes and publication. A prompt and an
// explanation may show media under DG-109's rules (D-064); which assets they name, and whether the caller may use
// them, is the store's check under the asset locks.
func (in Input) ValidateContent() error {
	errs := &ValidationError{}
	if !validPrompt(in) {
		errs.AddIn("promptContent", "Nội dung có định dạng không hợp lệ hoặc không khớp văn bản.",
			"The formatted text is not valid or does not match the plain text.")
	}
	if in.Type == FillBlank && hasProse(in.PromptContent) {
		validateGapBindings(in, errs)
	}
	if !validProse(in.ExplanationContent, in.Explanation) {
		errs.AddIn("explanationContent", "Lời giải có định dạng không hợp lệ hoặc không khớp văn bản.",
			"The formatted explanation is not valid or does not match its plain text.")
	}
	if invalid := errs.OrNil(); invalid != nil {
		return invalid
	}
	return nil
}

// ValidatePlacedAudio refuses a prompt whose audio marker has no question audio to place: the marker draws the
// question's own recording, with its policy and play count, and names none of its own (D-064 ruling 3). assetKind is
// the kind of the question's attached media, nil without one. A prompt that fails the media rules is left to
// ValidateContent.
func (in Input) ValidatePlacedAudio(assetKind *string) error {
	if !hasProse(in.PromptContent) {
		return nil
	}
	document, err := content.ParsePromptWithMedia(in.PromptContent)
	if err != nil || !document.PlacesQuestionAudio() || (in.MediaAssetID != nil && assetKind != nil && *assetKind == "audio") {
		return nil
	}
	errs := &ValidationError{}
	errs.AddIn("promptContent", "Vị trí âm thanh trong nội dung cần tệp âm thanh của câu hỏi.",
		"The audio placed in the text needs the question's own audio file.")
	return errs
}

func validPrompt(in Input) bool {
	if !hasProse(in.PromptContent) {
		return true
	}
	document, err := content.ParsePromptWithMedia(in.PromptContent)
	return err == nil && document.PlainText() == in.Prompt && (in.Type == FillBlank || len(document.GapIDs()) == 0)
}

func hasProse(raw json.RawMessage) bool {
	return len(raw) != 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validProse(raw json.RawMessage, text *string) bool {
	if !hasProse(raw) {
		return true
	}
	document, err := content.ParseExplanationWithMedia(raw)
	return err == nil && text != nil && document.PlainText() == *text
}
