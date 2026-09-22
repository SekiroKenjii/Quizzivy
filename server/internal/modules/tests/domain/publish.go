package domain

import (
	"fmt"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"strconv"
	"time"
)

// PublishManager holds the rules a draft must pass to become a version, and the totals frozen with it.
type PublishManager struct{}

// Totals sums the points the version is scored out of, frozen so the
// denominator on an old attempt cannot drift.
func (PublishManager) Totals(d DraftContent) (string, int) {
	var total float64
	count := 0
	for _, section := range d.Sections {
		for _, q := range section.Questions {
			total += parsePoints(q.Points)
			count++
		}
	}
	return formatPoints(total), count
}

// ValidateDraft runs §8's publish checks and returns every failure at once.
func (PublishManager) Validate(d DraftContent) error {
	var violations []Violation
	add := func(v Violation) { violations = append(violations, v) }

	for _, section := range d.Sections {
		if len(section.Questions) == 0 {
			add(Violation{
				Rule:      SectionNotEmpty,
				Message:   fmt.Sprintf("Phần %q chưa có câu hỏi nào.", section.Title),
				SectionID: section.ID,
			})
			continue
		}
		for _, q := range section.Questions {
			validateQuestion(section, q, add)
		}
	}

	if len(violations) > 0 {
		return &PublishValidationError{Violations: violations}
	}
	return nil
}

var Publishing PublishManager

// Rule identifies which publish check a violation came from. The set is
// closed and matches the contract's enum, so the builder can key its inline
// markers off it rather than off message text.
type Rule string

// Violation is one failed check, anchored to whatever the builder can highlight.
type Violation struct {
	Rule       Rule
	Message    string
	SectionID  string
	QuestionID string
}

const (
	PointsPositive         Rule = "points_positive"
	ChoiceHasCorrectOption Rule = "choice_has_correct_option"
	BlankHasAcceptedAnswer Rule = "blank_has_accepted_answer"
	BlankPlaceholdersMatch Rule = "blank_placeholders_match"
	AudioQuestionHasAsset  Rule = "audio_question_has_asset"
	OptionContentValid     Rule = "option_content_valid"
	SectionNotEmpty        Rule = "section_not_empty"
)

// PublishValidationError carries every violation at once.
type PublishValidationError struct{ Violations []Violation }

// PublishedVersion is the snapshot that was created.
type PublishedVersion struct {
	ID            string
	Version       int
	TotalPoints   string
	QuestionCount int
	PublishedAt   time.Time
	PublishedBy   string
}

// PublishRequest is one publish.
type PublishRequest struct {
	TestID    string
	ActorID   string
	IP        string
	UserAgent string
}

func validateQuestion(section DraftSection, q DraftQuestion, add func(Violation)) {
	anchor := func(rule Rule, message string) Violation {
		return Violation{Rule: rule, Message: message, SectionID: section.ID, QuestionID: q.SourceID}
	}

	if points, err := strconv.ParseFloat(q.Points, 64); err != nil || points <= 0 {
		add(anchor(PointsPositive, "Câu hỏi phải có điểm lớn hơn 0."))
	}

	if isChoice(q.Type) && !hasCorrectOption(q.Options) {
		add(anchor(ChoiceHasCorrectOption, "Câu hỏi trắc nghiệm cần ít nhất một phương án đúng."))
	}

	for _, option := range q.Options {
		in := questionsdomain.OptionInput{Text: option.Text, Content: option.Content}
		if in.ValidateContent() != nil {
			add(anchor(OptionContentValid, "Định dạng phương án không hợp lệ hoặc không khớp nội dung văn bản."))
		}
	}

	if q.Type == "fill_blank" {
		validateBlanks(q, anchor, add)
	}

	if q.AllowSeek != nil && !hasAudioAsset(q) {
		add(anchor(AudioQuestionHasAsset, "Câu hỏi có thiết lập nghe nhưng chưa đính kèm tệp âm thanh."))
	}
}

func hasCorrectOption(options []DraftOption) bool {
	for _, o := range options {
		if o.IsCorrect {
			return true
		}
	}
	return false
}

func hasAudioAsset(q DraftQuestion) bool {
	return q.MediaAssetID != nil && q.MediaAssetKind != nil && *q.MediaAssetKind == "audio"
}

func validateBlanks(q DraftQuestion, anchor func(Rule, string) Violation, add func(Violation)) {
	for _, b := range q.Blanks {
		if len(b.AcceptedAnswers) == 0 {
			add(anchor(BlankHasAcceptedAnswer,
				fmt.Sprintf("Chỗ trống %d chưa có đáp án được chấp nhận.", b.Ordinal)))
		}
	}

	inPrompt := questionsdomain.Questions.PromptPlaceholders(q.Prompt)
	promptSet := make(map[int]bool, len(inPrompt))
	for _, n := range inPrompt {
		promptSet[n] = true
	}
	blankSet := make(map[int]bool, len(q.Blanks))
	for _, b := range q.Blanks {
		blankSet[b.Ordinal] = true
	}

	for _, n := range inPrompt {
		if !blankSet[n] {
			add(anchor(BlankPlaceholdersMatch,
				fmt.Sprintf("Đề bài có {{%d}} nhưng thiếu chỗ trống %d.", n, n)))
		}
	}
	for _, b := range q.Blanks {
		if !promptSet[b.Ordinal] {
			add(anchor(BlankPlaceholdersMatch,
				fmt.Sprintf("Chỗ trống %d không có {{%d}} tương ứng trong đề bài.", b.Ordinal, b.Ordinal)))
		}
	}
}

func parsePoints(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func formatPoints(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func (e *PublishValidationError) Error() string {
	return fmt.Sprintf("publish: %d validation problem(s)", len(e.Violations))
}
