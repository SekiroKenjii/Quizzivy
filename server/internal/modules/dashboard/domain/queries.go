package domain

import (
	"time"
)

// ListQuery filters the cross-assignment attempt list the two queues are built from.
type ListQuery struct {
	Status         *string
	Flagged        *bool
	PendingGrading *bool
	Page           int
	Limit          int
}

// ActiveWindow bounds who counts as an active student: the dashboard is today's work queue.
const ActiveWindow = 7 * 24 * time.Hour
