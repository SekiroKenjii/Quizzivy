package domain

import (
	"bytes"
	"encoding/json"
	"quizzivy/internal/shared/content"
)

// ValidateContent checks the prose profiles and exact companion text before writes and publication.
func (in Input) ValidateContent() error {
	var fields []FieldError
	if holdsMedia(in.PromptContent, content.ParsePromptWithMedia) {
		fields = append(fields, FieldError{Field: "promptContent", Message: mediaNotYet})
	} else if !validPrompt(in) {
		fields = append(fields, FieldError{Field: "promptContent", Message: "Nội dung có định dạng không hợp lệ hoặc không khớp văn bản."})
	}
	if in.Type == FillBlank && hasProse(in.PromptContent) {
		validateGapBindings(in, func(field, message string) {
			fields = append(fields, FieldError{Field: field, Message: message})
		})
	}
	if holdsMedia(in.ExplanationContent, content.ParseExplanationWithMedia) {
		fields = append(fields, FieldError{Field: "explanationContent", Message: mediaNotYet})
	} else if !validProse(in.ExplanationContent, in.Explanation) {
		fields = append(fields, FieldError{Field: "explanationContent", Message: "Lời giải có định dạng không hợp lệ hoặc không khớp văn bản."})
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// mediaNotYet refuses an image or audio block in a prompt or an explanation, which the contract describes (DG-109)
// and the bank stores only once R6-13 validates ownership, the question's audio and the freeze (D-064, ruling 7).
const mediaNotYet = "Chưa thể chèn hình ảnh hoặc âm thanh vào nội dung câu hỏi."

func holdsMedia(raw json.RawMessage, parse func([]byte) (content.Document, error)) bool {
	if !hasProse(raw) {
		return false
	}
	document, err := parse(raw)
	return err == nil && document.HasMedia()
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
