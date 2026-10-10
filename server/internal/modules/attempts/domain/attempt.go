// Package domain is the attempt context: the Attempt aggregate (a student's
// sitting of one published version), the paper it is dealt, the answers and
// events it records, its score, and the teacher's review of it. Rules that
// belong to no single entity are managers: GradingManager, DealManager,
// TimelineManager and InterventionManager.
package domain

import (
	testsdomain "quizzivy/internal/modules/tests/domain"
	"time"
)

type Status string

type Attempt struct {
	ID             string
	AssignmentID   string
	StudentID      string
	TestVersionID  string
	AttemptNo      int
	Status         Status
	StartedAt      time.Time
	DeadlineAt     time.Time
	SubmittedAt    *time.Time
	GradedAt       *time.Time
	FocusLossCount int
	Flagged        bool
}

const (
	InProgress Status = "in_progress"
	Submitted  Status = "submitted"
	TimedOut   Status = "timed_out"
	Graded     Status = "graded"
	Voided     Status = "voided"
)

// Session is everything the engine needs to run authoritatively: the attempt,
// the paper in presentation order, and the identity of this tab.
type Session struct {
	RemainingAttempts int
	Attempt           Attempt
	TestTitle         string
	Sections          []Section
	Questions         []Question
	Groups            []testsdomain.PreviewGroup
	SessionID         string
	BeaconToken       string
	ServerTime        time.Time
	AudioPlays        map[string]int
	GroupAudioPlays   map[string]int
	Answers           map[string][]byte
	Integrity         Integrity
	// Superseded reports that the reader named a session the attempt no longer
	// has; SessionID is then the one it named and BeaconToken is empty.
	Superseded bool
}

// AttemptRecord carries the two columns Attempt deliberately does not: the seed is the
// server's business and the session id is handed over separately.
type AttemptRecord struct {
	Attempt
	SessionID string
	Seed      int64
}

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
	TestTitle        string
	Targeted         bool
}

// Tally answers the two questions the create path asks, which look like one
// and are not.
type Tally struct {
	Spent int
	Next  int
}

// Score is what a closed attempt is worth so far.
type Score struct {
	Earned        float64
	Total         float64
	PendingManual int
}

// SessionLiveWindow decides whether a resume supersedes a tab that was still
// open (`session_takeover`) or merely re-enters one that had gone -- a reload,
// a crash, a closed laptop -- which is `resume` alone.
const SessionLiveWindow = 2 * time.Minute

// Event kinds this package writes itself. The rest of §10.1's list arrives from
// the client and is never enumerated in Go -- see 00023 for why kind is not an
// enum.
const (
	KindResume          = "resume"
	KindSessionTakeover = "session_takeover"
)

// CanStartAt reports whether a student under these rules may start an attempt
// at now: the assignment is published, its window has opened, and neither its
// end nor an early close has passed. Rules are the student's own, so a window
// an override has lengthened or reopened is the one judged.
func (r Rules) CanStartAt(now time.Time) error {
	switch {
	case r.PublishedAt == nil:
		return ErrNotFound
	case now.Before(r.OpensAt), !now.Before(r.ClosesAt):
		return ErrAssignmentClosed
	case r.ClosedAt != nil && !now.Before(*r.ClosedAt):
		return ErrAssignmentClosed
	}
	return nil
}

// Deadline is the §9 rule, server-side and authoritative: a student gets their
// full duration unless the assignment closes first.
func (r Rules) Deadline(now time.Time) time.Time {
	full := now.Add(time.Duration(r.DurationMinutes) * time.Minute)
	if r.ClosesAt.Before(full) {
		return r.ClosesAt
	}
	return full
}
