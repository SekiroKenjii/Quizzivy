// Package domain is the authoring context: the Test aggregate with its sections
// and the questions they reference, the immutable Versions published from a
// draft, and PublishManager, the rules and totals a draft is frozen by.
package domain

import (
	"time"
)

// Test is a test with its DRAFT outline. Published content lives in versions.
type Test struct {
	ID             string
	Title          string
	Description    *string
	Status         Status
	CurrentVersion int
	TotalPoints    string
	QuestionCount  int
	AudioCount     int
	Sections       []Section
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Section is one part of the draft outline, with its questions in order.
type Section struct {
	ID           string
	Ordinal      int
	Title        string
	Instructions *string
	QuestionIDs  []string
}

type Status string

// StatusFacets is how many tests each status holds for one search.
type StatusFacets struct {
	All       int
	Draft     int
	Published int
	Archived  int
}

const (
	Draft     Status = "draft"
	Published Status = "published"
	Archived  Status = "archived"
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

func (s Status) valid() bool {
	switch s {
	case Draft, Published, Archived:
		return true
	}
	return false
}
