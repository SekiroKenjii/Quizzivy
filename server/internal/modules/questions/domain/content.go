package domain

import (
	"bytes"
	"encoding/json"
	"quizzivy/internal/shared/content"
)

// ValidateContent checks the prose profiles and exact companion text before writes and publication.
func (in Input) ValidateContent() error {
	errs := &ValidationError{}
	switch {
	case holdsMedia(in.PromptContent):
		refuseMedia(errs, "promptContent")
	case !validPrompt(in):
		errs.AddIn("promptContent", "Nội dung có định dạng không hợp lệ hoặc không khớp văn bản.",
			"The formatted text is not valid or does not match the plain text.")
	}
	if in.Type == FillBlank && hasProse(in.PromptContent) {
		validateGapBindings(in, errs)
	}
	switch {
	case holdsMedia(in.ExplanationContent):
		refuseMedia(errs, "explanationContent")
	case !validProse(in.ExplanationContent, in.Explanation):
		errs.AddIn("explanationContent", "Lời giải có định dạng không hợp lệ hoặc không khớp văn bản.",
			"The formatted explanation is not valid or does not match its plain text.")
	}
	if invalid := errs.OrNil(); invalid != nil {
		return invalid
	}
	return nil
}

// refuseMedia refuses an image or audio block inside a prompt or an
// explanation, which the bank does not store until R6-13 (D-064).
func refuseMedia(errs *ValidationError, field string) {
	errs.AddIn(field, "Chưa thể chèn hình ảnh hoặc âm thanh vào nội dung câu hỏi.",
		"Images and audio cannot be placed inside the question text yet.")
}

// holdsMedia reports whether raw is a document holding an image or audio block.
func holdsMedia(raw json.RawMessage) bool {
	if !hasProse(raw) {
		return false
	}
	document, err := content.Parse(raw)
	return err == nil && len(document.Assets()) > 0
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
