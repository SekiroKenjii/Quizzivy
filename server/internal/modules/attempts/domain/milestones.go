package domain

import (
	"context"
	"time"
)

// Milestones names what one write to an attempt brought about that people
// other than the student are told of. The zero value is a write that brought
// nothing about.
type Milestones struct {
	AttemptID string
	// HandedIn is set when the write closed the paper: the student submitted
	// it, its time ran out, or the integrity limit closed it.
	HandedIn bool
	// Flagged is set when the integrity policy, and not a teacher, flagged
	// the paper in this write for the first time.
	Flagged bool
	// Graded is set when a teacher declared the paper graded.
	Graded bool
}

// Any reports whether the write brought something about.
func (m Milestones) Any() bool {
	return m.HandedIn || m.Flagged || m.Graded
}

// Briefing is what a notice about a paper says that the paper does not: who
// sat it, which test, who may read it, and whether the student's score is
// shown now.
type Briefing struct {
	AssignmentID string
	StudentID    string
	StudentName  string
	Title        string
	FocusLost    int
	// ToGrade is set while an answer waits for a mark.
	ToGrade bool
	// ShowsResult is set when the assignment's review policy shows the
	// student's score at the time the briefing was read.
	ShowsResult bool
	// Readers are the teachers who reach the paper (visibility.PaperReaders)
	// and are enabled.
	Readers []string
}

// Briefings reads the briefing of an attempt as of now. It answers
// ErrNotFound for an attempt that does not exist or was voided.
type Briefings interface {
	Briefing(ctx context.Context, attemptID string, now time.Time) (Briefing, error)
}
