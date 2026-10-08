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
	Skills         []string
	Assignments    AssignmentCounts
	Sections       []Section
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// AssignmentCounts is how many non-draft assignments name any version of a
// test, by derived status: Live counts the open ones.
type AssignmentCounts struct {
	Live      int
	Scheduled int
	Closed    int
}

// Section is one part of the draft outline, with its questions in order.
type Section struct {
	ID           string
	Ordinal      int
	Title        string
	Instructions *string
	QuestionIDs  []string
	Units        []SectionUnit
}

// SectionUnit identifies a standalone question or complete owned group in authored order.
type SectionUnit struct {
	Kind string
	ID   string
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
// AssignmentCount is every assignment that names it, drafts included.
// TestUpdatedAt is set only on the Version that Publish returns: the test's
// update time after the publish moved it.
type Version struct {
	ID              string
	Version         int
	TotalPoints     string
	QuestionCount   int
	AudioCount      int
	ManualCount     int
	PublishedAt     time.Time
	PublishedBy     string
	AssignmentCount int
	ChangeNote      *string
	TestUpdatedAt   *time.Time
}

func (s Status) valid() bool {
	switch s {
	case Draft, Published, Archived:
		return true
	}
	return false
}
