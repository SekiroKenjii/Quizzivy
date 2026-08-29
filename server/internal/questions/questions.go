// Package questions owns the §8 question bank: the authored questions and the
// per-type detail that hangs off them.
package questions

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Type string

const (
	SingleChoice   Type = "single_choice"
	MultipleChoice Type = "multiple_choice"
	TrueFalse      Type = "true_false"
	FillBlank      Type = "fill_blank"
	ShortAnswer    Type = "short_answer"
)

// choiceTypes are the types whose answer is a set of options. Named once so the
// validation rules below cannot disagree about which types have options.
func (t Type) isChoice() bool {
	return t == SingleChoice || t == MultipleChoice || t == TrueFalse
}

func (t Type) valid() bool {
	switch t {
	case SingleChoice, MultipleChoice, TrueFalse, FillBlank, ShortAnswer:
		return true
	}
	return false
}

var (
	ErrNotFound = errors.New("questions: not found")
	// ErrReferenced is a delete refused because a draft test outline still uses
	// the question. 409, not 403: the teacher has every right to it, it is
	// simply not deletable while something depends on it.
	ErrReferenced = errors.New("questions: referenced by a draft test outline")
	ErrBadCursor  = errors.New("questions: malformed cursor")
)

// AudioPolicy is §7's `audio?: AudioPolicy`, present iff the asset is audio.
type AudioPolicy struct {
	// MaxPlays nil means unlimited (§11.4). Nullable rather than absent, which
	// is why the database constrains it separately from the two booleans.
	MaxPlays                  *int
	AllowSeek                 bool
	ShowTranscriptAfterSubmit bool
}

type Option struct {
	ID        string
	Ordinal   int
	Text      string
	IsCorrect bool
}

type Blank struct {
	ID              string
	Ordinal         int
	AcceptedAnswers []string
	CaseSensitive   bool
}

// Question is a bank row with its children. Media is carried as the id and kind
// pair the composite FK needs [D-05]; the handler resolves the asset itself.
type Question struct {
	ID             string
	Type           Type
	Prompt         string
	MediaAssetID   *string
	MediaAssetKind *string
	Audio          *AudioPolicy
	Transcript     *string
	Options        []Option
	Blanks         []Blank
	Points         string // numeric(8,2) as text -- never a float (§13.2)
	Explanation    *string
	SampleAnswer   *string
	Tags           []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Input is a create or update body, already parsed but not yet validated.
type Input struct {
	Type         Type
	Prompt       string
	MediaAssetID *string
	Audio        *AudioPolicy
	Transcript   *string
	Options      []OptionInput
	Blanks       []BlankInput
	Points       string
	Explanation  *string
	SampleAnswer *string
	Tags         []string
}

type OptionInput struct {
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

// FieldError names the field a rule failed on, so the client can put the
// message next to the input rather than at the top of the form.
type FieldError struct {
	Field   string
	Message string
}

// ValidationError carries every failure at once. Returning the first would make
// fixing a form a series of round trips.
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "questions: " + strings.Join(parts, "; ")
}

// placeholderPattern matches `{{1}}`, `{{2}}` -- §7's fill_blank markers, and
// the resolution recorded in 40-open-items.md. 1-indexed, matching
// blanks[].ordinal.
var placeholderPattern = regexp.MustCompile(`\{\{(\d+)\}\}`)

// PromptPlaceholders returns the distinct ordinals a prompt refers to, sorted.
func PromptPlaceholders(prompt string) []int {
	seen := map[int]bool{}
	for _, m := range placeholderPattern.FindAllStringSubmatch(prompt, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 1 {
			// Unparseable or 0-indexed: not a placeholder this system defines,
			// so it is prose that happens to look like one.
			continue
		}
		seen[n] = true
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// Validate enforces the cross-field rules QuestionInput's description lists --
// the ones "a single schema cannot express". The request validator has already
// checked types, lengths and enums by the time this runs.
//
// Every rule here is also enforced at publish (§8), on the snapshot rather than
// the draft. That is not redundancy for its own sake: the bank is editable, so a
// question can be made invalid after it was accepted.
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

	validateOptions(in, add)
	validateBlanks(in, add)
	validateMedia(in, assetKind, add)

	// §7: short_answer only, and admin-only at read time.
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
	if !in.Type.isChoice() {
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
		if strings.TrimSpace(o.Text) == "" {
			add(fmt.Sprintf("options[%d].text", i), "Nội dung phương án không được để trống.")
		}
		if o.IsCorrect {
			correct++
		}
	}
	// The rule that makes a question gradeable at all. Without it the bank
	// accepts a question no answer can score on, and it is discovered by a
	// student mid-test.
	if correct == 0 {
		add("options", "Cần ít nhất một phương án đúng.")
	}
	if in.Type == SingleChoice && correct > 1 {
		add("options", "Câu hỏi một đáp án chỉ được có một phương án đúng.")
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
	seen := map[int]bool{}
	for i, b := range in.Blanks {
		if b.Ordinal < 1 {
			add(fmt.Sprintf("blanks[%d].ordinal", i), "Số thứ tự chỗ trống bắt đầu từ 1.")
		}
		if seen[b.Ordinal] {
			add(fmt.Sprintf("blanks[%d].ordinal", i), "Số thứ tự chỗ trống bị trùng.")
		}
		seen[b.Ordinal] = true
		if len(b.AcceptedAnswers) == 0 {
			add(fmt.Sprintf("blanks[%d].acceptedAnswers", i), "Cần ít nhất một đáp án được chấp nhận.")
		}
		for j, a := range b.AcceptedAnswers {
			if strings.TrimSpace(a) == "" {
				add(fmt.Sprintf("blanks[%d].acceptedAnswers[%d]", i, j),
					"Đáp án không được để trống.")
			}
		}
	}

	// The prompt's placeholders and the blank ordinals must be the same set.
	// A blank with no placeholder is unreachable; a placeholder with no blank
	// renders as literal `{{2}}` to the student and can never be answered.
	inPrompt := PromptPlaceholders(in.Prompt)
	promptSet := map[int]bool{}
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

	// [D-04] in application terms, and the same biconditional the database
	// enforces: the policy is present exactly when the asset is audio.
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
