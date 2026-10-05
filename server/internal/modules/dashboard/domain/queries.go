package domain

import (
	"quizzivy/internal/shared/access"
	"time"
)

// ListQuery filters the cross-assignment attempt list the two queues are built
// from, over the attempts on assignments Scope reaches that visibility.Papers
// shows it; a zero Scope matches nothing.
type ListQuery struct {
	Status         *string
	Flagged        *bool
	PendingGrading *bool
	Page           int
	Limit          int
	Scope          access.Scope
}

// ActiveWindow bounds who counts as an active student: the dashboard is today's work queue.
const ActiveWindow = 7 * 24 * time.Hour

// HomeQuery fixes the scope, application clock and calendar zone for the new readings.
type HomeQuery struct {
	Scope access.Scope
	Now   time.Time
	Zone  string
	Days  int
}

// SummaryQuery fixes the scope and permission for the flagged destination.
type SummaryQuery struct {
	Scope            access.Scope
	CanReviewFlagged bool
}
