package domain

import (
	"errors"
	"strings"
)

var (
	ErrNotFound         = errors.New("assignments: not found")
	ErrTestNotPublished = errors.New("assignments: test version is not published")
	ErrVersionLocked    = errors.New("assignments: attempts exist")
	// ErrAssignmentLocked is a change to the version, the duration or the
	// number of attempts of an assignment that is open.
	ErrAssignmentLocked = errors.New("assignments: open, so the test and timing are locked")
)

var (
	ErrNotClosed    = errors.New("assignments: not closed")
	ErrBlankReason  = errors.New("assignments: reason is blank")
	ErrClosesInPast = errors.New("assignments: closes_at is not ahead")
	// ErrClosed is an extension of a window that has already closed: the
	// assignment, or the student whose close is extended, is reopened instead.
	ErrClosed = errors.New("assignments: closed, so it is reopened and not extended")
)

// NotTargetedError names the ids of an override request that are not students
// of the assignment the actor reaches. A student who does not exist and one
// the actor may not know exist are the same to it.
type NotTargetedError struct {
	StudentIDs []string
}

func (e *NotTargetedError) Error() string {
	return "assignments: not students of this assignment: " + strings.Join(e.StudentIDs, ", ")
}

// ErrForbidden covers not targeted, not published and not found alike. Which
// assignments exist is not a student's to enumerate.
var ErrForbidden = errors.New("assignments: not this student's")

var (
	ErrReferenced  = errors.New("assignment: retained history or assignments reference this resource")
	ErrNotArchived = errors.New("assignment: deactivate or close before deleting")
)
