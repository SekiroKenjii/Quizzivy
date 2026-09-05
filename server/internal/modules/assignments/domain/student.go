package domain

import (
	"errors"
	"time"
)

// ErrForbidden covers not targeted, not published and not found alike. Which
// assignments exist is not a student's to enumerate.
var ErrForbidden = errors.New("assignments: not this student's")

// StudentCard is what a student may know about an assignment before opening
// it: §9's card, and nothing about anyone else's work. No targets, no counts,
// no roster -- those are the teacher's projection (Assignment), and the two
// are separate types so a field added to one cannot leak through the other.
type StudentCard struct {
	ID        string
	TestTitle string
	// ClassName is set only when exactly one targeted class contains them.
	ClassName     *string
	OpensAt       time.Time
	ClosesAt      time.Time
	ClosedAt      *time.Time
	PublishedAt   *time.Time
	DurationMin   int
	MaxAttempts   int
	QuestionCount int
	TotalPoints   float64
	AttemptsUsed  int
	// HasLiveAttempt means resumable: in progress and before its deadline.
	HasLiveAttempt bool
	// LiveDeadlineAt is non-nil exactly when HasLiveAttempt is true.
	LiveDeadlineAt *time.Time
	// LastAttemptID is the most recent non-voided attempt, live or finished.
	LastAttemptID *string
	// LastSubmittedAt is nil while that attempt is still live.
	LastSubmittedAt *time.Time
	Score           *Score
}

type Score struct {
	Earned        float64
	Total         float64
	PendingManual int
}

// StudentDetail is the intro page: the card plus every policy §10.2 has to
// state before the clock starts.
type StudentDetail struct {
	StudentCard
	// TeacherName is the assignment's author.
	TeacherName *string
	Review      Review
	Integrity   Integrity
	HasAudio    bool
	// ShowsTranscript is true when any listening question releases one.
	ShowsTranscript bool
	AudioMaxPlays   *int
}

// StudentSections is §9's home, already sorted into its three lists.
type StudentSections struct {
	DueNow    []StudentCard
	Upcoming  []StudentCard
	Completed []StudentCard
}
