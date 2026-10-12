// Package domain is the reporting context: read models over the other
// modules' data for the teacher's home. It owns no aggregate.
package domain

import (
	"errors"
	"time"
)

// Summary is the teacher home with legacy figures and its calendar readings.
type Summary struct {
	Home
	NewestFlaggedAttempt *FlaggedAttempt
	ClosingSoon          int
	WaitingStudents      int
	OldestWaitingAt      *time.Time
	TotalStudents        int
	NextClosing          *ClosingAssignment
	OpenAssignments      int
	AwaitingGrading      int
	ActiveStudents       int
	FlaggedAttempts      int
	Recent               []Recent
}

// FlaggedAttempt identifies one reachable flagged paper and its assignment.
type FlaggedAttempt struct{ AssignmentID, AttemptID string }

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

// ErrNotificationsUnavailable means the nav summary has no notifications port.
var ErrNotificationsUnavailable = errors.New("dashboard: notifications unavailable")

// Home is the calendar and activity reading of the teacher dashboard.
type Home struct {
	TakingNow      TakingNow
	Submissions    Submissions
	Today          []Today
	RecentActivity []Activity
}

// TakingNow counts distinct students and assignments before their attempt deadlines.
type TakingNow struct {
	Students, Assignments int
	AssignmentID          *string
}

// Submissions is a complete calendar series and the graded mean in that range.
type Submissions struct {
	Days           []Day
	Total          int
	AveragePercent *int
}

// Day is a local calendar date and its handed-in paper count.
type Day struct {
	Date  string
	Count int
}

// Today is one published assignment opening or effective closing.
type Today struct {
	Kind                string
	At                  time.Time
	AssignmentID, Title string
	NotSubmitted        int
}

// Activity is one attempt state or a student-like class join by code.
type Activity struct {
	Kind                 string
	At                   time.Time
	StudentName, Subject string
	Flagged              bool
}

// Nav is the caller's permission-aware shell counts.
type Nav struct {
	LiveAssignments, AnswersToGrade *int
	UnreadNotifications             int
}
