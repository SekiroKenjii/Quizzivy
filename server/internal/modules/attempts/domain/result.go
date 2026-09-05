package domain

import (
	"errors"
)

var ErrAttemptInProgress = errors.New("attempts: attempt is still in progress")

// ReviewPolicy is what the assignment lets a student see afterwards (§9).
type ReviewPolicy struct {
	ShowScore          bool
	ShowCorrectAnswers bool
	ShowExplanations   bool
}

// BlankAnswer is one canonical accepted answer, never the full list.
type BlankAnswer struct {
	BlankID string
	Answer  string
}

// ResultQuestion is the post-submission view of one question. Every revealing
// field is nil unless the policy released it -- and the query that reads it
// never selected the column when it did not, so there is nothing to strip.
type ResultQuestion struct {
	Question
	Answer         []byte
	Earned         *float64
	PendingManual  bool
	GraderComment  *string
	CorrectOptions []string
	CorrectAnswers []BlankAnswer
	Explanation    *string
	Transcript     *string
	AudioPlaysUsed *int
}

type Result struct {
	Attempt     Attempt
	Score       *Score
	Review      ReviewPolicy
	TestTitle   string
	MaxAttempts int
	Questions   []ResultQuestion
}
