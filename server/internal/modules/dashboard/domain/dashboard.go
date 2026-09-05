package domain

import (
	"context"
	"time"

	"quizzivy/internal/shared/paging"
)

// ActiveWindow bounds who counts as an active student: the dashboard is today's work queue.
const ActiveWindow = 7 * 24 * time.Hour

// Recent is one attempt as the teacher's queues list it.
type Recent struct {
	ID            string
	StudentID     string
	StudentName   string
	AssignmentID  string
	TestTitle     string
	Status        string
	SubmittedAt   *time.Time
	PendingManual int
	Flagged       bool
}

// Summary is the admin home in one reading: the four counts and the latest attempts.
type Summary struct {
	OpenAssignments int
	AwaitingGrading int
	ActiveStudents  int
	FlaggedAttempts int
	Recent          []Recent
}

// ListQuery filters the cross-assignment attempt list the two queues are built from.
type ListQuery struct {
	Status         *string
	Flagged        *bool
	PendingGrading *bool
	Page           int
	Limit          int
}

// Repository reads the dashboard's aggregates.
type Repository interface {
	Summary(ctx context.Context) (Summary, error)
	List(ctx context.Context, q ListQuery) ([]Recent, paging.Page, error)
}
