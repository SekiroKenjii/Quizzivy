package publish

import (
	"errors"
	"fmt"
	"time"
)

// Rule identifies which publish check a violation came from. The set is
// closed and matches the contract's enum, so the builder can key its inline
// markers off it rather than off message text.
type Rule string

const (
	PointsPositive         Rule = "points_positive"
	ChoiceHasCorrectOption Rule = "choice_has_correct_option"
	BlankHasAcceptedAnswer Rule = "blank_has_accepted_answer"
	BlankPlaceholdersMatch Rule = "blank_placeholders_match"
	AudioQuestionHasAsset  Rule = "audio_question_has_asset"
	SectionNotEmpty        Rule = "section_not_empty"
)

// Violation is one failed check, anchored to whatever the builder can highlight.
type Violation struct {
	Rule       Rule
	Message    string
	SectionID  string
	QuestionID string
}

// ValidationError carries every violation at once.
//
// Returning the first would make publishing a long test a sequence of attempts,
// each surfacing one more problem. §8 wants them marked inline together.
type ValidationError struct{ Violations []Violation }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("publish: %d validation problem(s)", len(e.Violations))
}

var (
	ErrNotFound = errors.New("publish: test not found")
	// ErrNoContent is a test with no sections at all. Distinct from an empty
	// section: there is nothing to anchor a violation to.
	ErrNoContent = errors.New("publish: test has no sections")
)

// Version is the snapshot that was created.
type Version struct {
	ID            string
	Version       int
	TotalPoints   string
	QuestionCount int
	PublishedAt   time.Time
	PublishedBy   string
}
