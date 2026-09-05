package domain

import (
	"strings"
)

// CleanReason rejects a reason that is only whitespace: the schema's minLength
// cannot tell "   " from a reason, and a blank one defeats the audit AttemptRecord.
func (InterventionManager) CleanReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", ErrBlankReason
	}
	return reason, nil
}

// InterventionManager holds the rules for the teacher's interventions on a
// live attempt: what a reason must look like, what may be extended or voided.
type InterventionManager struct{}

// Request is who is intervening and from where, for the audit AttemptRecord every
// intervention writes (§13.4).
type Request struct {
	ActorID   string
	IP        string
	UserAgent string
}

// Reason records how an attempt ended. It is the contract's `reason`, kept out
// of the status column: status says what still has to happen to the attempt,
// this says what stopped it.
type Reason string

const (
	Manual       Reason = "manual"
	TimerExpired Reason = "timer_expired"
	AutoSubmit   Reason = "auto_submit"
)
