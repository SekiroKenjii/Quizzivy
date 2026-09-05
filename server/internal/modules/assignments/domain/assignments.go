package domain

import (
	"time"
)

type Status string

const (
	Draft     Status = "draft"
	Scheduled Status = "scheduled"
	Open      Status = "open"
	Closed    Status = "closed"
)

// StatusAt is D-18's pure function: no scheduler, no stale row.
//
// The draft case does not weaken that. Publishing is an act by the teacher, not
// a timestamp arriving, so nothing has to flip a row when a clock passes -- the
// window rule reads exactly as it did once PublishedAtOf exists.
func StatusAt(now time.Time, PublishedAtOf *time.Time, opensAt, closesAt time.Time, ClosedAtOf *time.Time) Status {
	if PublishedAtOf == nil {
		return Draft
	}
	if ClosedAtOf != nil && !now.Before(*ClosedAtOf) {
		return Closed
	}
	switch {
	case now.Before(opensAt):
		return Scheduled
	case now.Before(closesAt):
		return Open
	default:
		return Closed
	}
}

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

type ListInput struct {
	Status *Status
	// ClassID narrows the list to assignments that target the class (G-12).
	ClassID *string
	Page    int
	Limit   int
}

// Facets are the list's tab counts: every assignment by its status right now,
// ignoring the status filter, so picking one tab does not zero the others.
type Facets struct {
	All, Draft, Scheduled, Open, Closed int
}
