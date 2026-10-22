package domain

import (
	"bytes"
	"encoding/json"
	"quizzivy/internal/shared/content"
)

// ValidateContent checks the prose profiles and exact companion text before writes and publication.
func (in Input) ValidateContent() error {
	var fields []FieldError
	if !validPrompt(in) {
		fields = append(fields, FieldError{Field: "promptContent", Message: "Nội dung có định dạng không hợp lệ hoặc không khớp văn bản."})
	}
	if in.Type == FillBlank && hasProse(in.PromptContent) {
		validateGapBindings(in, func(field, message string) {
			fields = append(fields, FieldError{Field: field, Message: message})
		})
	}
	if !validProse(in.ExplanationContent, in.Explanation) {
		fields = append(fields, FieldError{Field: "explanationContent", Message: "Lời giải có định dạng không hợp lệ hoặc không khớp văn bản."})
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
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
