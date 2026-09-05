package domain

import (
	"time"
)

type SaveInput struct {
	AttemptID string
	StudentID string
	SessionID string
	Answers   []Answer
	Events    []Event
}

// SaveResult reports what actually landed. Saved can be lower than the number
// of answers submitted -- see Postgres.Save for why that is not an error.
type SaveResult struct {
	SavedAt time.Time
	Saved   int
	// Dropped names the answers that did not land.
	Dropped []string
}

// FlushInput carries the events and exactly one credential.
type FlushInput struct {
	AttemptID string
	SessionID string
	Events    []Event

	// StudentID is set when the request arrived with a verified access token.
	StudentID   string
	BeaconToken string
}

type CreateInput struct {
	AssignmentID  string
	TestVersionID string
	StudentID     string
	AttemptNo     int
	SessionID     string
	Seed          int64
	BeaconHash    []byte
	StartedAt     time.Time
	DeadlineAt    time.Time
}

type ResumeInput struct {
	AttemptID  string
	SessionID  string
	BeaconHash []byte
	Now        time.Time
}
