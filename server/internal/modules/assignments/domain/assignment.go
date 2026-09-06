// Package domain is the assignment context: the Assignment aggregate (one
// published version given to classes and students for a window), its review and
// integrity policies as value objects, the student's view of it, and
// ScheduleManager for everything the window decides.
package domain

import (
	"time"
)

type Assignment struct {
	ID                  string
	TestID              string
	TestVersionID       string
	TestVersion         int
	TestTitle           string
	Classes             []ClassRef
	Students            []StudentRef
	OpensAt             time.Time
	ClosesAt            time.Time
	ClosedAt            *time.Time
	PublishedAt         *time.Time
	UpdatedAt           time.Time
	DurationMin         int
	MaxAttempts         int
	ShuffleQ            bool
	ShuffleO            bool
	Review              Review
	Integrity           Integrity
	SubmittedCount      int
	TargetCount         int
	FlaggedCount        int
	PendingGradingCount int
}

type Status string

const (
	Draft     Status = "draft"
	Scheduled Status = "scheduled"
	Open      Status = "open"
	Closed    Status = "closed"
)

type Review struct {
	ShowScore, ShowCorrectAnswers, ShowExplanations bool
}

type Integrity struct {
	RequireFullscreen bool
	BlockCopyPaste    bool
	MaxFocusLoss      int
	OnLimitExceeded   string
	MinAwayMs         int
}

// ClassRef is a targeted class, its name and its live member count.
type ClassRef struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	StudentCount int    `json:"studentCount"`
}

// StudentRef is a student targeted by name.
type StudentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Facets are the list's tab counts: every assignment by its status right now,
// ignoring the status filter, so picking one tab does not zero the others.
type Facets struct {
	All, Draft, Scheduled, Open, Closed int
}
