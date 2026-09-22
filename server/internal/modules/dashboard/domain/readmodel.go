// Package domain is the reporting context: read models over the other
// modules' data for the teacher's home. It owns no aggregate.
package domain

import (
	"time"
)

// Summary is the admin home in one reading: the four counts and the latest attempts.
type Summary struct {
	ClosingSoon     int
	WaitingStudents int
	OldestWaitingAt *time.Time
	TotalStudents   int
	NextClosing     *ClosingAssignment
	OpenAssignments int
	AwaitingGrading int
	ActiveStudents  int
	FlaggedAttempts int
	Recent          []Recent
}

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

// ClosingAssignment is the nearest open assignment due within 24 hours.
type ClosingAssignment struct {
	ID             string
	Title          string
	ClosesAt       time.Time
	SubmittedCount int
	TargetCount    int
}
