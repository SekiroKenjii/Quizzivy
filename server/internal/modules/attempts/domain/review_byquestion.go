package domain

import (
	"errors"
	"time"
)

var ErrQuestionNotOnPaper = errors.New("review: question is not on this paper")

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

// ByQuestion is G-04's read: one question with its place on the paper, the
// paper's other manual questions to walk to, and every handed-in answer.
type ByQuestion struct {
	Question    ReviewQuestion
	PublishedAt time.Time
	Number      int
	Count       int
	ManualIDs   []string
	Items       []QuestionAnswer
}
