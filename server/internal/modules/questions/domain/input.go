package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/validation"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxMediaAltLength is the most characters the alt text of a question's image
// holds: the bound ContentImage.alt has and the check on both media_alt columns.
const MaxMediaAltLength = 1000

// Input is a create or update body, already parsed but not yet validated.
type Input struct {
	PromptContent      json.RawMessage
	ExplanationContent json.RawMessage
	Type               Type
	Level              *Level
	Skill              *Skill
	Prompt             string
	MediaAssetID       *string
	MediaAlt           *string
	Audio              *AudioPolicy
	Transcript         *string
	Options            []OptionInput
	Blanks             []BlankInput
	Points             string
	Explanation        *string
	SampleAnswer       *string
	Tags               []string
}

// WriteRequest is a write to the bank by ActorID. It reaches only ActorID's own
// questions and binds only assets ActorID may read, unless All, the actor's
// scope.all, is set; anything else answers exactly as a missing id does. On a
// create, OwnerID owns the new question instead of ActorID; only a Word-import
// commit sets it, and a group member still follows its group.
type WriteRequest struct {
	ID        string
	Input     Input
	ActorID   string
	OwnerID   string
	All       bool
	IP        string
	UserAgent string
}

// Scope is the scope the request reads and binds under: ActorID's own, or
// everyone's with All.
func (r WriteRequest) Scope() access.Scope {
	return access.Scope{UserID: r.ActorID, All: r.All}
}

// WriteInput is a create or an update, depending on whether ID is set. An
// update or delete of a bank question needs ActorID to own it unless All is
// set; a group member's write is authorized by its group and ignores both.
// OwnerID applies to a create only, as on WriteRequest.
type WriteInput struct {
	ID             string
	Input          Input
	MediaAssetKind *string
	ActorID        string
	OwnerID        string
	All            bool
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
	GapID           *string
	ID              *string
	Ordinal         int
	AcceptedAnswers []string
	CaseSensitive   bool
}

// ListInput selects a page of the bank. Scope limits it to the caller's own
// questions, or every teacher's under scope.all; a zero Scope matches nothing.
type ListInput struct {
	Types    []Type
	Levels   []Level
	Skills   []Skill
	TagMatch string
	Tags     []string
	HasAudio *bool
	Query    string
	Page     int
	Limit    int
	Scope    access.Scope
}

// Validate enforces question invariants for HTTP, internal writes and publication.
func (in Input) Validate(assetKind *string) error {
	errs := &ValidationError{}

	if !in.Type.valid() {
		errs.AddIn("type", "Loại câu hỏi không hợp lệ.", "The question type is not valid.")
		return errs
	}
	if in.Level != nil && !validLevel(*in.Level) {
		errs.AddIn("level", "Trình độ không hợp lệ.", "The level is not valid.")
	}
	if in.Skill != nil && !validSkill(*in.Skill) {
		errs.AddIn("skill", "Kỹ năng không hợp lệ.", "The skill is not valid.")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		errs.AddIn("prompt", "Nội dung câu hỏi không được để trống.", "The question text cannot be empty.")
	}
	if _, valid := PointUnits(in.Points); !valid {
		errs.AddIn("points", "Điểm phải lớn hơn 0, không quá 999999,99 và có tối đa hai chữ số thập phân.", "Points must be more than 0, at most 999999.99, with at most two decimal places.")
	}

	if err := in.ValidateContent(); err != nil {
		errs.Fields = append(errs.Fields, err.(*ValidationError).Fields...)
	}
	validateOptions(in, errs)
	validateBlanks(in, errs)
	validateMedia(in, assetKind, errs)
	validateMediaAlt(in, assetKind, errs)

	if in.SampleAnswer != nil && *in.SampleAnswer != "" && in.Type != ShortAnswer {
		errs.AddIn("sampleAnswer", "Đáp án mẫu chỉ dùng cho câu trả lời ngắn.", "Only a short answer has a sample answer.")
	}
	for i, tag := range in.Tags {
		if strings.TrimSpace(tag) == "" {
			errs.AddIn(fmt.Sprintf("tags[%d]", i), "Thẻ không được để trống.", "A tag cannot be empty.")
		}
	}

	if invalid := errs.OrNil(); invalid != nil {
		return invalid
	}
	return nil
}

func validateOptions(in Input, errs *ValidationError) {
	if !in.Type.IsChoice() {
		if len(in.Options) > 0 {
			errs.AddIn("options", "Loại câu hỏi này không có phương án chọn.", "This question type has no options.")
		}
		return
	}

	if len(in.Options) < 2 {
		errs.AddIn("options", "Cần ít nhất hai phương án.", "Add at least two options.")
	}
	correct := 0
	for i, o := range in.Options {
		validateOptionText(i, o, errs)
		if o.IsCorrect {
			correct++
		}
	}
	if correct == 0 {
		errs.AddIn("options", "Cần ít nhất một phương án đúng.", "Mark at least one option correct.")
	}
	if (in.Type == SingleChoice || in.Type == TrueFalse) && correct > 1 {
		errs.AddIn("options", "Câu hỏi một đáp án hoặc đúng/sai chỉ được có một phương án đúng.", "A single choice or true/false question has exactly one correct option.")
	}
	if in.Type == TrueFalse && len(in.Options) != 2 {
		errs.AddIn("options", "Câu đúng/sai phải có đúng hai phương án.", "A true/false question has exactly two options.")
	}
}

func validateBlanks(in Input, errs *ValidationError) {
	if in.Type != FillBlank {
		if len(in.Blanks) > 0 {
			errs.AddIn("blanks", "Loại câu hỏi này không có chỗ trống.", "This question type has no blanks.")
		}
		return
	}
	if len(in.Blanks) == 0 {
		errs.AddIn("blanks", "Cần ít nhất một chỗ trống.", "Add at least one blank.")
	}
	ordinals := validateEachBlank(in, errs)
	if !hasProse(in.PromptContent) {
		validateBlankOrdinalsMatchPrompt(in, ordinals, errs)
		for _, blank := range in.Blanks {
			if blank.GapID != nil {
				errs.AddIn("blanks", "Liên kết chỗ trống chỉ dùng với nội dung có định dạng.", "Only formatted text links a blank to a gap.")
			}
		}
	}
}

func validateGapBindings(in Input, errs *ValidationError) {
	document, err := content.ParseQuestionPrompt(in.PromptContent)
	if err != nil {
		return
	}
	if len(document.GapIDs()) == 0 || len(in.Blanks) == 0 {
		errs.AddIn("blanks", "Cần ít nhất một chỗ trống với đáp án tương ứng.", "Add at least one blank with its answers.")
		return
	}
	expected := make(map[string]bool)
	for _, id := range document.GapIDs() {
		expected[id] = true
	}
	for _, blank := range in.Blanks {
		if blank.GapID == nil || !expected[*blank.GapID] {
			errs.AddIn("blanks", "Mỗi chỗ trống phải liên kết đúng một vị trí trong nội dung.", "Each blank must link to exactly one gap in the text.")
			continue
		}
		delete(expected, *blank.GapID)
	}
	if len(expected) != 0 {
		errs.AddIn("blanks", "Có chỗ trống trong nội dung chưa có đáp án tương ứng.", "A gap in the text has no answers yet.")
	}
}

func validateEachBlank(in Input, errs *ValidationError) map[int]bool {
	seen := map[int]bool{}
	for i, b := range in.Blanks {
		if b.Ordinal < 1 || b.Ordinal > 32767 {
			errs.AddIn(fmt.Sprintf("blanks[%d].ordinal", i), "Số thứ tự chỗ trống phải từ 1 đến 32767.", "A blank's number must be from 1 to 32767.")
		}
		if seen[b.Ordinal] {
			errs.AddIn(fmt.Sprintf("blanks[%d].ordinal", i), "Số thứ tự chỗ trống bị trùng.", "Two blanks have the same number.")
		}
		seen[b.Ordinal] = true
		validateBlankAnswers(i, b, errs)
	}
	return seen
}

func validateBlankAnswers(i int, b BlankInput, errs *ValidationError) {
	if len(b.AcceptedAnswers) == 0 {
		errs.AddIn(fmt.Sprintf("blanks[%d].acceptedAnswers", i), "Cần ít nhất một đáp án được chấp nhận.", "Add at least one accepted answer.")
	}
	for j, a := range b.AcceptedAnswers {
		if strings.TrimSpace(a) == "" {
			errs.AddIn(fmt.Sprintf("blanks[%d].acceptedAnswers[%d]", i, j), "Đáp án không được để trống.", "An answer cannot be empty.")
		}
	}
}

func validateBlankOrdinalsMatchPrompt(in Input, seen map[int]bool, errs *ValidationError) {
	inPrompt := Questions.PromptPlaceholders(in.Prompt)
	promptSet := make(map[int]bool, len(inPrompt))
	for _, n := range inPrompt {
		promptSet[n] = true
	}
	for n := range seen {
		if !promptSet[n] {
			errs.AddIn("blanks", fmt.Sprintf("Chỗ trống %d không có {{%d}} tương ứng trong đề bài.", n, n),
				fmt.Sprintf("Blank %d has no {{%d}} in the question text.", n, n))
		}
	}
	for _, n := range inPrompt {
		if !seen[n] {
			errs.AddIn("prompt", fmt.Sprintf("Đề bài có {{%d}} nhưng thiếu chỗ trống %d.", n, n),
				fmt.Sprintf("The question text has {{%d}} but no blank %d.", n, n))
		}
	}
}

func validateMedia(in Input, assetKind *string, errs *ValidationError) {
	hasAudio := assetKind != nil && *assetKind == "audio"

	switch {
	case hasAudio && in.Audio == nil:
		errs.AddIn("audio", "Câu hỏi có tệp âm thanh cần thiết lập nghe.", "A question with audio needs its listening settings.")
	case !hasAudio && in.Audio != nil:
		errs.AddIn("audio", "Chỉ câu hỏi có tệp âm thanh mới có thiết lập nghe.", "Only a question with audio has listening settings.")
	}
	if in.Audio != nil && in.Audio.MaxPlays != nil && *in.Audio.MaxPlays < 1 {
		errs.AddIn("audio.maxPlays", "Số lần nghe phải lớn hơn 0.", "The number of plays must be more than 0.")
	}
	if in.Transcript != nil && *in.Transcript != "" && !hasAudio {
		errs.AddIn("transcript", "Chỉ câu hỏi có tệp âm thanh mới có lời thoại.", "Only a question with audio has a transcript.")
	}
	if in.MediaAssetID == nil && in.Audio != nil {
		errs.AddIn("audio", "Chưa chọn tệp âm thanh.", "No audio file is chosen.")
	}
}

func validateMediaAlt(in Input, assetKind *string, errs *ValidationError) {
	if in.MediaAlt == nil {
		return
	}
	switch {
	case assetKind == nil || *assetKind != "image":
		errs.AddIn("mediaAlt", "Chỉ câu hỏi có hình ảnh mới có văn bản thay thế.", "Only a question with an image has alt text.")
	case strings.TrimSpace(*in.MediaAlt) == "":
		errs.AddIn("mediaAlt", "Văn bản thay thế không được để trống.", "The alt text cannot be empty.")
	case utf8.RuneCountInString(*in.MediaAlt) > MaxMediaAltLength:
		errs.AddIn("mediaAlt", "Văn bản thay thế không được dài quá 1000 ký tự.", "The alt text cannot be longer than 1000 characters.")
	case !utf8.ValidString(*in.MediaAlt) || strings.ContainsRune(*in.MediaAlt, 0):
		errs.AddIn("mediaAlt", "Văn bản thay thế chứa ký tự không hợp lệ.", "The alt text holds a character that is not allowed.")
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

func validateOptionText(index int, option OptionInput, errs *ValidationError) {
	if err := option.ValidateContent(); err != nil {
		errs.AddIn(fmt.Sprintf("options[%d].content", index), "Định dạng phương án không hợp lệ hoặc không khớp nội dung văn bản.", "The option's formatting is not valid or does not match its text.")
	}
	if strings.TrimSpace(option.Text) == "" {
		errs.AddIn(fmt.Sprintf("options[%d].text", index), "Nội dung phương án không được để trống.", "An option's text cannot be empty.")
	}
}
