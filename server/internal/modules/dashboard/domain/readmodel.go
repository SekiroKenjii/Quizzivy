// Package domain is the reporting context: read models over the other
// modules' data for the teacher's home. It owns no aggregate.
package domain

import (
	"time"
)

// Summary is the admin home in one reading: the four counts and the latest attempts.
type Summary struct {
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
