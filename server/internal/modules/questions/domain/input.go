package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/validation"
	"strings"
	"time"
)

// Input is a create or update body, already parsed but not yet validated.
type Input struct {
	PromptContent      json.RawMessage
	ExplanationContent json.RawMessage
	Type               Type
	Prompt             string
	MediaAssetID       *string
	Audio              *AudioPolicy
	Transcript         *string
	Options            []OptionInput
	Blanks             []BlankInput
	Points             string
	Explanation        *string
	SampleAnswer       *string
	Tags               []string
}

type WriteRequest struct {
	ID        string
	Input     Input
	ActorID   string
	IP        string
	UserAgent string
}

// WriteInput is a create or an update, depending on whether ID is set.
type WriteInput struct {
	ID             string
	Input          Input
	MediaAssetKind *string
	ActorID        string
	Now            time.Time
	IP             string
	UserAgent      string
}

type OptionInput struct {
	Content   json.RawMessage
	ID        *string
	Text      string
	IsCorrect bool
}

type BlankInput struct {
	ID              *string
	Ordinal         int
	AcceptedAnswers []string
	CaseSensitive   bool
}

// ListInput selects a page of the bank.
type ListInput struct {
	Types    []Type
	Tags     []string
	HasAudio *bool
	Query    string
	Page     int
	Limit    int
}

// Validate enforces question invariants for HTTP, internal writes and publication.
func (in Input) Validate(assetKind *string) error {
	var errs []FieldError
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }

	if !in.Type.valid() {
		add("type", "Loại câu hỏi không hợp lệ.")
		return &ValidationError{Fields: errs}
	}
	if strings.TrimSpace(in.Prompt) == "" {
		add("prompt", "Nội dung câu hỏi không được để trống.")
	}
	if _, valid := PointUnits(in.Points); !valid {
		add("points", "Điểm phải lớn hơn 0, không quá 999999,99 và có tối đa hai chữ số thập phân.")
	}

	if err := in.ValidateContent(); err != nil {
		errs = append(errs, err.(*ValidationError).Fields...)
	}
	validateOptions(in, add)
	validateBlanks(in, add)
	validateMedia(in, assetKind, add)

	if in.SampleAnswer != nil && *in.SampleAnswer != "" && in.Type != ShortAnswer {
		add("sampleAnswer", "Đáp án mẫu chỉ dùng cho câu trả lời ngắn.")
	}
	for i, tag := range in.Tags {
		if strings.TrimSpace(tag) == "" {
			add(fmt.Sprintf("tags[%d]", i), "Thẻ không được để trống.")
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}

func validateOptions(in Input, add func(string, string)) {
	if !in.Type.IsChoice() {
		if len(in.Options) > 0 {
			add("options", "Loại câu hỏi này không có phương án chọn.")
		}
		return
	}

	if len(in.Options) < 2 {
		add("options", "Cần ít nhất hai phương án.")
	}
	correct := 0
	for i, o := range in.Options {
		validateOptionText(i, o, add)
		if o.IsCorrect {
			correct++
		}
	}
	if correct == 0 {
		add("options", "Cần ít nhất một phương án đúng.")
	}
	if (in.Type == SingleChoice || in.Type == TrueFalse) && correct > 1 {
		add("options", "Câu hỏi một đáp án hoặc đúng/sai chỉ được có một phương án đúng.")
	}
	if in.Type == TrueFalse && len(in.Options) != 2 {
		add("options", "Câu đúng/sai phải có đúng hai phương án.")
	}
}

func validateBlanks(in Input, add func(string, string)) {
	if in.Type != FillBlank {
		if len(in.Blanks) > 0 {
			add("blanks", "Loại câu hỏi này không có chỗ trống.")
		}
		return
	}
	if len(in.Blanks) == 0 {
		add("blanks", "Cần ít nhất một chỗ trống.")
	}
	validateBlankOrdinalsMatchPrompt(in, validateEachBlank(in, add), add)
}

func validateEachBlank(in Input, add func(string, string)) map[int]bool {
	seen := map[int]bool{}
	for i, b := range in.Blanks {
		if b.Ordinal < 1 {
			add(fmt.Sprintf("blanks[%d].ordinal", i), "Số thứ tự chỗ trống bắt đầu từ 1.")
		}
		if seen[b.Ordinal] {
			add(fmt.Sprintf("blanks[%d].ordinal", i), "Số thứ tự chỗ trống bị trùng.")
		}
		seen[b.Ordinal] = true
		validateBlankAnswers(i, b, add)
	}
	return seen
}

func validateBlankAnswers(i int, b BlankInput, add func(string, string)) {
	if len(b.AcceptedAnswers) == 0 {
		add(fmt.Sprintf("blanks[%d].acceptedAnswers", i), "Cần ít nhất một đáp án được chấp nhận.")
	}
	for j, a := range b.AcceptedAnswers {
		if strings.TrimSpace(a) == "" {
			add(fmt.Sprintf("blanks[%d].acceptedAnswers[%d]", i, j), "Đáp án không được để trống.")
		}
	}
}

func validateBlankOrdinalsMatchPrompt(in Input, seen map[int]bool, add func(string, string)) {
	inPrompt := Questions.PromptPlaceholders(in.Prompt)
	promptSet := make(map[int]bool, len(inPrompt))
	for _, n := range inPrompt {
		promptSet[n] = true
	}
	for n := range seen {
		if !promptSet[n] {
			add("blanks", fmt.Sprintf("Chỗ trống %d không có {{%d}} tương ứng trong đề bài.", n, n))
		}
	}
	for _, n := range inPrompt {
		if !seen[n] {
			add("prompt", fmt.Sprintf("Đề bài có {{%d}} nhưng thiếu chỗ trống %d.", n, n))
		}
	}
}

func validateMedia(in Input, assetKind *string, add func(string, string)) {
	hasAudio := assetKind != nil && *assetKind == "audio"

	switch {
	case hasAudio && in.Audio == nil:
		add("audio", "Câu hỏi có tệp âm thanh cần thiết lập nghe.")
	case !hasAudio && in.Audio != nil:
		add("audio", "Chỉ câu hỏi có tệp âm thanh mới có thiết lập nghe.")
	}
	if in.Audio != nil && in.Audio.MaxPlays != nil && *in.Audio.MaxPlays < 1 {
		add("audio.maxPlays", "Số lần nghe phải lớn hơn 0.")
	}
	if in.Transcript != nil && *in.Transcript != "" && !hasAudio {
		add("transcript", "Chỉ câu hỏi có tệp âm thanh mới có lời thoại.")
	}
	if in.MediaAssetID == nil && in.Audio != nil {
		add("audio", "Chưa chọn tệp âm thanh.")
	}
}

type (
	FieldError      = validation.Field
	ValidationError = validation.Error
)

// ValidateContent checks the optional versioned option document and its exact text projection.
func (o OptionInput) ValidateContent() error {
	if len(o.Content) == 0 || bytes.Equal(bytes.TrimSpace(o.Content), []byte("null")) {
		return nil
	}
	document, err := content.ParseOption(o.Content)
	if err != nil || document.PlainText() != o.Text || strings.TrimSpace(o.Text) == "" {
		return content.ErrInvalidDocument
	}
	return nil
}

func validateOptionText(index int, option OptionInput, add func(string, string)) {
	if err := option.ValidateContent(); err != nil {
		add(fmt.Sprintf("options[%d].content", index), "Định dạng phương án không hợp lệ hoặc không khớp nội dung văn bản.")
	}
	if strings.TrimSpace(option.Text) == "" {
		add(fmt.Sprintf("options[%d].text", index), "Nội dung phương án không được để trống.")
	}
}
