package domain

import (
	"encoding/json"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"strings"
	"time"
)

type Result struct {
	SharedContext *SharedReviewContext
	Attempt       Attempt
	Score         *Score
	Review        ReviewPolicy
	TestTitle     string
	MaxAttempts   int
	Questions     []ResultQuestion
}

// Review is G-03's data in one read.
type Review struct {
	SharedContext *SharedReviewContext
	Attempt       Attempt
	Score         Score
	TestTitle     string
	MaxAttempts   int
	// PublishedAt is when the version froze; the questions carry no clock of their own.
	PublishedAt time.Time
	// TeacherNote is G-05's private note: the teacher's, never the student's.
	TeacherNote *string
	Questions   []ReviewQuestion
	Answers     map[string]ReviewAnswer
	AudioPlays  map[string]int
}

// ReviewQuestion is a frozen version question with everything the grader may see.
type ReviewQuestion struct {
	GroupID            string
	PromptContent      json.RawMessage
	ExplanationContent json.RawMessage
	ID                 string
	Type               string
	Prompt             string
	Points             float64
	Media              *Media
	Audio              *AudioPolicy
	Transcript         *string
	Explanation        *string
	SampleAnswer       *string
	Options            []ReviewOption
	Blanks             []ReviewBlank
}

type ReviewOption struct {
	Content   json.RawMessage
	ID        string
	Ordinal   int
	Text      string
	IsCorrect bool
}

type ReviewBlank struct {
	GapID         *string
	ID            string
	Ordinal       int
	CaseSensitive bool
	Accepted      []string
}

// ReviewAnswer is one saved answer and what it has earned so far.
type ReviewAnswer struct {
	Payload        []byte
	AutoScore      *float64
	ManualScore    *float64
	RequiresManual bool
	GraderComment  *string
}

// ReviewPolicy is what the assignment lets a student see afterwards (§9).
type ReviewPolicy struct {
	ShowScore          bool
	ShowCorrectAnswers bool
	ShowExplanations   bool
}

// ResultQuestion is the post-submission view of one question. Every revealing
// field is nil unless the policy released it -- and the query that reads it
// never selected the column when it did not, so there is nothing to strip.
type ResultQuestion struct {
	ExplanationContent json.RawMessage
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

// GradeItem is one manual mark: the question, the points, and a comment the
// student will read.
type GradeItem struct {
	QuestionID string
	Points     float64
	Comment    *string
}

// GradeItemError names an item that could not be marked and why -- `not_on_paper`,
// `unanswered` or `above_ceiling`.
type GradeItemError struct {
	QuestionID string
	Reason     string
}

type GradeValidationError struct{ Items []GradeItemError }

// ByQuestion is G-04's read: one question with its place on the paper, the
// paper's other manual questions to walk to, and every handed-in answer.
type ByQuestion struct {
	VersionID     string
	SharedContext *SharedReviewContext
	Question      ReviewQuestion
	PublishedAt   time.Time
	Number        int
	Count         int
	ManualIDs     []string
	Items         []QuestionAnswer
}

// SharedReviewContext carries frozen material and transcripts allowed for this review surface.
type SharedReviewContext struct {
	Groups      []testsdomain.PreviewGroup
	Transcripts map[string]string
	AudioPlays  map[string]int
}

// QuestionAnswer is one paper's answer to the question being graded.
type QuestionAnswer struct {
	AttemptID     string
	StudentID     string
	StudentName   string
	AttemptNo     int
	Payload       []byte
	ManualScore   *float64
	GraderComment *string
}

func (e *GradeValidationError) Error() string {
	parts := make([]string, len(e.Items))
	for i, it := range e.Items {
		parts[i] = it.QuestionID + ": " + it.Reason
	}
	return "review: " + strings.Join(parts, "; ")
}
