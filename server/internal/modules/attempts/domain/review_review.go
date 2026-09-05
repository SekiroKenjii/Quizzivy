package domain

import (
	"errors"
	"time"
)

var (
	ErrPaperNotFound     = errors.New("review: attempt not found")
	ErrPaperInProgress   = errors.New("review: attempt is still in progress")
	ErrPaperVoided       = errors.New("review: attempt is voided")
	ErrGradingIncomplete = errors.New("review: a manual answer is still ungraded")
)

type ReviewOption struct {
	ID        string
	Ordinal   int
	Text      string
	IsCorrect bool
}

type ReviewBlank struct {
	ID            string
	Ordinal       int
	CaseSensitive bool
	Accepted      []string
}

// ReviewQuestion is a frozen version question with everything the grader may see.
type ReviewQuestion struct {
	ID           string
	Type         string
	Prompt       string
	Points       float64
	Media        *Media
	Audio        *AudioPolicy
	Transcript   *string
	Explanation  *string
	SampleAnswer *string
	Options      []ReviewOption
	Blanks       []ReviewBlank
}

// ReviewAnswer is one saved answer and what it has earned so far.
type ReviewAnswer struct {
	Payload        []byte
	AutoScore      *float64
	ManualScore    *float64
	RequiresManual bool
	GraderComment  *string
}

// Review is G-03's data in one read.
type Review struct {
	Attempt     Attempt
	Score       Score
	TestTitle   string
	MaxAttempts int
	// PublishedAt is when the version froze; the questions carry no clock of their own.
	PublishedAt time.Time
	// TeacherNote is G-05's private note: the teacher's, never the student's.
	TeacherNote *string
	Questions   []ReviewQuestion
	Answers     map[string]ReviewAnswer
	AudioPlays  map[string]int
}
