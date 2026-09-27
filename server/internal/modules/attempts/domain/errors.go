package domain

import (
	"errors"
	"time"
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

var ErrGroupContextUnavailable = errors.New("attempts: group context reader unavailable")

var ErrUnsupportedDeliveryVersion = errors.New("attempts: unsupported delivery version")

var ErrPlayIDConflict = errors.New("attempts: play ID already belongs to another recording")

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

// TimerGrace is how early a timer_expired submission may arrive and still
// close the attempt: clocks disagree by a little, never by more.
const TimerGrace = 5 * time.Second

// DeadlineNotReachedError refuses a timer_expired submission that arrived
// more than TimerGrace before the attempt's deadline, which moved after the
// client last saved. DeadlineAt is the deadline to wait for.
type DeadlineNotReachedError struct {
	DeadlineAt time.Time
}

func (e *DeadlineNotReachedError) Error() string {
	return "attempts: deadline not reached until " + e.DeadlineAt.UTC().Format(time.RFC3339)
}

// MaintenanceWindow is a maintenance window as the attempts module sees it.
type MaintenanceWindow struct {
	StartsAt time.Time
	EndsAt   time.Time
}

// MaintenanceScheduledError refuses to start an attempt that would run into a
// maintenance window. Resuming an attempt never meets it.
type MaintenanceScheduledError struct {
	Window MaintenanceWindow
}

func (e *MaintenanceScheduledError) Error() string {
	return "attempts: a maintenance window starts at " + e.Window.StartsAt.UTC().Format(time.RFC3339)
}
