package domain

import (
	"errors"
)

var (
	ErrNotFound = errors.New("attempts: not found")
	// ErrForbidden covers both "not your attempt" and "not assigned to you".
	ErrForbidden        = errors.New("attempts: not yours")
	ErrAssignmentClosed = errors.New("attempts: assignment is not open")
	ErrLimitReached     = errors.New("attempts: attempt limit reached")

	// ErrSessionSuperseded is how the tab that lost finds out.
	ErrSessionSuperseded = errors.New("attempts: session superseded")
	ErrDeadlinePassed    = errors.New("attempts: deadline passed")
	ErrAttemptClosed     = errors.New("attempts: attempt is no longer in progress")

	ErrBeaconExpired = errors.New("attempts: beacon token has expired")
)

// ErrRaceLost means a concurrent create won. The caller resumes rather than
// erroring: from the student's side a double-tap produced one attempt, which is
// exactly what they wanted.
var ErrRaceLost = errors.New("attempts: concurrent create won")

var ErrAttemptInProgress = errors.New("attempts: attempt is still in progress")

var (
	ErrBlankReason   = errors.New("attempts: reason is blank")
	ErrAttemptVoided = errors.New("attempts: attempt is voided")
)

var ErrTimelineNotFound = errors.New("integrity: attempt not found")

var ErrQuestionNotOnPaper = errors.New("review: question is not on this paper")

var (
	ErrPaperNotFound     = errors.New("review: attempt not found")
	ErrPaperInProgress   = errors.New("review: attempt is still in progress")
	ErrPaperVoided       = errors.New("review: attempt is voided")
	ErrGradingIncomplete = errors.New("review: a manual answer is still ungraded")
)
