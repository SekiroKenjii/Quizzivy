package domain

import (
	"time"
)

// Score is what a closed attempt is worth so far.
type Score struct {
	Earned        float64
	Total         float64
	PendingManual int
}

// MonitorRow is one targeted student on G-02, whether or not they have started.
type MonitorRow struct {
	StudentID string
	FullName  string
	// State is `not_started`, or the attempt's status.
	State          string
	AttemptID      *string
	AttemptNo      *int
	StartedAt      *time.Time
	DeadlineAt     *time.Time
	SubmittedAt    *time.Time
	AnsweredCount  *int
	Score          *Score
	FocusLossCount *int
	Flagged        bool
	AudioOverLimit bool
}

// Monitor is the §8 monitor screen's data: the roster, each with the attempt
// that stands for them.
type Monitor struct {
	ServerTime    time.Time
	QuestionCount int
	Rows          []MonitorRow
}
