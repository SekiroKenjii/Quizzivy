package domain

import (
	"quizzivy/internal/shared/access"
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
// intervention writes (§13.4). An intervention reaches only an attempt on an
// assignment ActorID reaches, unless All, the actor's scope.all, is set;
// another teacher's attempt answers exactly as a missing one does.
type Request struct {
	ActorID   string
	All       bool
	IP        string
	UserAgent string
}

// Scope is the scope the intervention reaches under: ActorID's own, or
// everyone's with All.
func (r Request) Scope() access.Scope {
	return access.Scope{UserID: r.ActorID, All: r.All}
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
