package domain

import (
	"bytes"
	"encoding/json"
	"quizzivy/internal/shared/content"
)

// ValidateContent checks the prose profiles and exact companion text before writes and publication.
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

func validPrompt(in Input) bool {
	if !hasProse(in.PromptContent) {
		return true
	}
	document, err := content.ParseQuestionPrompt(in.PromptContent)
	return err == nil && document.PlainText() == in.Prompt && (in.Type == FillBlank || len(document.GapIDs()) == 0)
}

func hasProse(raw json.RawMessage) bool {
	return len(raw) != 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validProse(raw json.RawMessage, text *string) bool {
	if !hasProse(raw) {
		return true
	}
	document, err := content.ParseQuestion(raw)
	return err == nil && text != nil && document.PlainText() == *text
}
