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
	// ReleasesAt is when the result is released, set only while an
	// after-close release is withheld.
	ReleasesAt *time.Time
	// ClassAverage is the class's mean best score as a percent, set only when
	// ReviewManager.ShowsAverage allows it.
	ClassAverage *float64
	// Sections are the paper's parts in test order, whatever the review
	// policy: the attempt already showed them.
	Sections  []Section
	Questions []ResultQuestion
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
	SelectCount        *int
	MaxWords           *int
	MatchAllowReuse    *bool
	Options            []ReviewOption
	Blanks             []ReviewBlank
	MatchItems         []ReviewMatchItem
	Rubric             []RubricCriterion
}

// ReviewMatchItem is one matching item with its key, which the grader sees.
type ReviewMatchItem struct {
	ID              string
	Key             string
	Text            string
	CorrectOptionID string
	Ordinal         int
}

// RubricCriterion is one of an essay's frozen rubric criteria.
type RubricCriterion struct {
	ID      string
	Label   string
	Ordinal int
}

type ReviewOption struct {
	Content   json.RawMessage
	ID        string
	Key       string
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
	Band           *float64
	GraderComment  *string
	Criteria       []CriterionBand
	RequiresManual bool
}

// ReviewPolicy is what the assignment lets a student see afterwards (§9).
type ReviewPolicy struct {
	ShowScore          bool
	ShowCorrectAnswers bool
	ShowExplanations   bool
	Release            Release
	ShowClassAverage   bool
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
	Skill          *string
	Band           *float64
	CorrectMatches []MatchPair
	Criteria       []ResultCriterion
	// ExplanationAssets are the media the released explanation shows
	// (DG-109), once each in document order; empty whenever the explanation
	// is withheld, because they are read from it.
	ExplanationAssets []string
}

// MatchPair is one matching item and the choice that answers it.
type MatchPair struct {
	ItemID   string
	OptionID string
}

// ResultCriterion is one rubric criterion of a graded essay and the band it
// was given.
type ResultCriterion struct {
	ID    string
	Label string
	Band  float64
}

// GradeItem is one manual mark: the question, its points or, for an essay,
// its band or the band of each of its rubric criteria, and a comment the
// student will read.
type GradeItem struct {
	Points     *float64
	Band       *float64
	Comment    *string
	QuestionID string
	Criteria   []CriterionBand
}

// CriterionBand is the band one rubric criterion of an essay was given.
type CriterionBand struct {
	CriterionID string
	Band        float64
}

// GradeItemError names an item that could not be marked and why -- `not_on_paper`,
// `unanswered` or `above_ceiling`.
type GradeItemError struct {
	QuestionID string
	Reason     string
}

type GradeValidationError struct{ Items []GradeItemError }

// AnswerRemovedError names the items of a grade whose answers were removed
// under the retention policy: they cannot be marked, and nothing was saved.
type AnswerRemovedError struct{ QuestionIDs []string }

func (e *AnswerRemovedError) Error() string {
	return "review: answers removed under the retention policy: " + strings.Join(e.QuestionIDs, ", ")
}

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
	Band          *float64
	GraderComment *string
	Criteria      []CriterionBand
}

func (e *GradeValidationError) Error() string {
	parts := make([]string, len(e.Items))
	for i, it := range e.Items {
		parts[i] = it.QuestionID + ": " + it.Reason
	}
	return "review: " + strings.Join(parts, "; ")
}

// AnswerRemoved reports whether a stored payload is the marker the retention
// sweep leaves in place of an answer (retention.RemovedAnswer): the response is
// gone and cannot be shown or graded, and its scores stand.
func AnswerRemoved(payload []byte) bool {
	var marker struct {
		Removed bool `json:"removed"`
		Type    any  `json:"type"`
	}
	return json.Unmarshal(payload, &marker) == nil && marker.Removed && marker.Type == nil
}
