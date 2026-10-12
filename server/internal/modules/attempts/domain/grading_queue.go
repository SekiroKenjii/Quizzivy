package domain

import (
	"quizzivy/internal/shared/access"
	"time"
)

// GradingQueueQuery selects pending saved answers within the existing paper reach.
type GradingQueueQuery struct {
	Scope        access.Scope
	Mode         string
	AssignmentID *string
	StudentID    *string
}

// GradingQueue is complete filtered counts and a bounded ordered answer prefix.
type GradingQueue struct {
	Groups           []GradingQueueGroup
	Items            []GradingQueueItem
	AnswersRemaining int
	StudentsWaiting  int
}

// GradingQueueGroup identifies represented work with its complete filtered count.
type GradingQueueGroup struct {
	Key       string
	Kind      string
	Label     string
	Sub       string
	Remaining int
}

// GradingQueueItem carries one saved answer and its frozen teacher question.
type GradingQueueItem struct {
	AttemptID       string
	AssignmentID    string
	AssignmentTitle string
	StudentID       string
	StudentName     string
	QuestionNumber  int
	VersionID       string
	PublishedAt     time.Time
	Question        ReviewQuestion
	Answer          ReviewAnswer
	SharedContext   *SharedReviewContext
}
