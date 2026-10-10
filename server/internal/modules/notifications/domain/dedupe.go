package domain

import (
	"strconv"
	"time"
)

const (
	// SubmittedBucket is how long papers handed in on one assignment merge
	// into one notification for a teacher.
	SubmittedBucket = 15 * time.Minute
	// DueLookback is how far back a due-time item is still written: one whose
	// moment is older than this was missed, and is not made up.
	DueLookback = 7 * 24 * time.Hour
	// DueEvery is the least time between two materialisations for one user.
	DueEvery = 5 * time.Minute
)

// Lead names how long before a close a due-soon notification falls.
type Lead string

const (
	// LeadDay is a day before the close.
	LeadDay Lead = "24h"
	// LeadHour is an hour before the close.
	LeadHour Lead = "1h"
)

// SubmittedKey is the dedupe key of the papers handed in on an assignment
// during the fifteen minutes at falls in.
func SubmittedKey(assignmentID string, at time.Time) string {
	return "submitted:" + assignmentID + ":" + strconv.FormatInt(at.Unix()/int64(SubmittedBucket/time.Second), 10)
}

// FlaggedKey is the dedupe key of the flag on one attempt.
func FlaggedKey(attemptID string) string {
	return "flagged:" + attemptID
}

// JoinedKey is the dedupe key of one student joining one class.
func JoinedKey(classID, studentID string) string {
	return "joined:" + classID + ":" + studentID
}

// ExtendedKey is the dedupe key of an assignment's close being moved for one
// student: a later move replaces the earlier one.
func ExtendedKey(assignmentID string) string {
	return "extended:" + assignmentID
}

// OpenedKey is the dedupe key of an assignment opening for one student.
func OpenedKey(assignmentID string) string {
	return "opened:" + assignmentID
}

// DueSoonKey is the dedupe key of the reminder lead before a student's close.
// It names the close, so a close that moves earns its reminders again.
func DueSoonKey(assignmentID string, closes time.Time, lead Lead) string {
	return "due_soon:" + assignmentID + ":" + strconv.FormatInt(closes.Unix(), 10) + ":" + string(lead)
}

// ClosingKey is the dedupe key of the warning that an assignment closes
// within the hour. It names the close, as DueSoonKey does.
func ClosingKey(assignmentID string, closes time.Time) string {
	return "closing:" + assignmentID + ":" + strconv.FormatInt(closes.Unix(), 10)
}

// ReadyKey is the dedupe key of one attempt's result being ready.
func ReadyKey(attemptID string) string {
	return "ready:" + attemptID
}
