package domain

import (
	"errors"
	"time"
)

// Rules is what starting an attempt needs from the assignment: whether this
// student may start at all, and under what constraints if so.
type Rules struct {
	TestVersionID    string
	OpensAt          time.Time
	ClosesAt         time.Time
	ClosedAt         *time.Time
	PublishedAt      *time.Time
	DurationMinutes  int
	MaxAttempts      int
	ShuffleQuestions bool
	ShuffleOptions   bool
	Integrity        Integrity
	Targeted         bool
}

// Deadline is the §9 rule, server-side and authoritative: a student gets their
// full duration unless the assignment closes first.
//
// 40-open-items.md P3 settles the other direction -- once started, deadline_at
// wins and the student finishes even if closes_at passes mid-attempt. That is
// why this is computed once at creation and never recomputed on resume.
func (r Rules) Deadline(now time.Time) time.Time {
	full := now.Add(time.Duration(r.DurationMinutes) * time.Minute)
	if r.ClosesAt.Before(full) {
		return r.ClosesAt
	}
	return full
}

// AttemptRecord carries the two columns Attempt deliberately does not: the seed is the
// server's business and the session id is handed over separately.
type AttemptRecord struct {
	Attempt
	SessionID string
	Seed      int64
}

// Tally answers the two questions the create path asks, which look like one
// and are not.
type Tally struct {
	Spent int
	Next  int
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

// ErrRaceLost means a concurrent create won. The caller resumes rather than
// erroring: from the student's side a double-tap produced one attempt, which is
// exactly what they wanted.
var ErrRaceLost = errors.New("attempts: concurrent create won")

type ResumeInput struct {
	AttemptID  string
	SessionID  string
	BeaconHash []byte
	Now        time.Time
}
