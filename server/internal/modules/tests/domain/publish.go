package domain

import (
	"fmt"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"time"
)

// PublishManager holds the rules a draft must pass to become a version, and the totals frozen with it.
type PublishManager struct{}

// Totals sums the points the version is scored out of, frozen so the
// denominator on an old attempt cannot drift.
func (PublishManager) Totals(d DraftContent) (string, int) {
	var total int64
	count := 0
	for _, section := range d.Sections {
		for _, q := range section.Questions {
			units, _ := questionsdomain.PointUnits(q.Points)
			total += units
			count++
		}
	}
	return fmt.Sprintf("%d.%02d", total/100, total%100), count
}

// Validate runs the shared question rules and assessment-level publication invariants.
func (PublishManager) Validate(d DraftContent) error {
	var violations []Violation
	add := func(v Violation) { violations = append(violations, v) }

	if len(d.Sections) == 0 {
		add(Violation{Rule: SectionNotEmpty, Message: "Đề cần ít nhất một phần có câu hỏi."})
	}
	var total int64
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
			units, _ := questionsdomain.PointUnits(q.Points)
			total += units
		}
	}

	if total > 99_999_999 {
		add(Violation{Rule: TotalPointsValid, Message: "Tổng điểm của đề không được vượt quá 999999,99."})
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
	QuestionContentValid   Rule = "question_content_valid"
	SectionNotEmpty        Rule = "section_not_empty"
	QuestionValid          Rule = "question_valid"
	TotalPointsValid       Rule = "total_points_valid"
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

func (e *PublishValidationError) Error() string {
	return fmt.Sprintf("publish: %d validation problem(s)", len(e.Violations))
}
