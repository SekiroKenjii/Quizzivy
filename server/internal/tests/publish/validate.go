package publish

import (
	"fmt"
	"strconv"

	"quizzivy/internal/questions"
)

// Validate runs §8's publish checks and returns every failure at once.
//
// These overlap with the bank's own validation on purpose. The bank stays
// editable after a question is accepted, so a question can be made invalid
// between authoring and publishing -- and publish is the last point at which a
// student can be spared it.
func Validate(d Draft) error {
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
		return &ValidationError{Violations: violations}
	}
	return nil
}

func validateQuestion(section Section, q Question, add func(Violation)) {
	anchor := func(rule Rule, message string) Violation {
		return Violation{Rule: rule, Message: message, SectionID: section.ID, QuestionID: q.SourceID}
	}

	if points, err := strconv.ParseFloat(q.Points, 64); err != nil || points <= 0 {
		add(anchor(PointsPositive, "Câu hỏi phải có điểm lớn hơn 0."))
	}

	if isChoice(q.Type) && !hasCorrectOption(q.Options) {
		add(anchor(ChoiceHasCorrectOption, "Câu hỏi trắc nghiệm cần ít nhất một phương án đúng."))
	}

	if q.Type == "fill_blank" {
		validateBlanks(q, anchor, add)
	}

	if q.AllowSeek != nil && !hasAudioAsset(q) {
		add(anchor(AudioQuestionHasAsset, "Câu hỏi có thiết lập nghe nhưng chưa đính kèm tệp âm thanh."))
	}
}

func hasCorrectOption(options []Option) bool {
	for _, o := range options {
		if o.IsCorrect {
			return true
		}
	}
	return false
}

func hasAudioAsset(q Question) bool {
	return q.MediaAssetID != nil && q.MediaAssetKind != nil && *q.MediaAssetKind == "audio"
}

func validateBlanks(q Question, anchor func(Rule, string) Violation, add func(Violation)) {
	for _, b := range q.Blanks {
		if len(b.AcceptedAnswers) == 0 {
			add(anchor(BlankHasAcceptedAnswer,
				fmt.Sprintf("Chỗ trống %d chưa có đáp án được chấp nhận.", b.Ordinal)))
		}
	}

	inPrompt := questions.PromptPlaceholders(q.Prompt)
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
