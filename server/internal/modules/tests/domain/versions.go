package domain

import (
	"time"
)

// Version is one published snapshot, newest first in a history.
type Version struct {
	ID            string
	Version       int
	TotalPoints   string
	QuestionCount int
	AudioCount    int
	ManualCount   int
	PublishedAt   time.Time
	PublishedBy   string
}
