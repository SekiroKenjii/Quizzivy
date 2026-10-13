package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrPaused is a teacher who may not use AI because an Admin paused it.
	ErrPaused = errors.New("ai: the teacher's AI is paused")
	// ErrExhausted is a reservation the pool or the teacher's allowance cannot cover. Exhausted carries which.
	ErrExhausted = errors.New("ai: the credits are used up")
	// ErrReservationGone is a settlement for a reservation that is no longer held, returned by the sweep or settled already.
	ErrReservationGone = errors.New("ai: the reservation is no longer held")
	// ErrUnknownFeature is a reservation for a feature this binary does not meter.
	ErrUnknownFeature = errors.New("ai: unknown feature")
	// ErrInvalidCeiling is a reservation of no credits, or of more than MaxCeiling.
	ErrInvalidCeiling = errors.New("ai: the ceiling is outside what a call may reserve")
	// ErrNotATeacher is an allowance for a user who does not exist, is disabled, or is a student.
	ErrNotATeacher = errors.New("ai: no such teacher")
	// ErrAllowanceBelowUse is an allowance set lower than the teacher has already used or holds this month.
	ErrAllowanceBelowUse = errors.New("ai: the allowance is below what the teacher has used")
	// ErrRequestsOff is a request for credits while the centre's rule is to pause a teacher who runs out.
	ErrRequestsOff = errors.New("ai: the centre does not take requests for more credits")
	// ErrRequestPending is a second request in a month while the first is undecided.
	ErrRequestPending = errors.New("ai: a request is already waiting")
	// ErrRequestNotFound is a decision about a request that does not exist.
	ErrRequestNotFound = errors.New("ai: no such request")
	// ErrRequestDecided is a decision about a request that was decided already.
	ErrRequestDecided = errors.New("ai: the request was decided already")
	// ErrRequestExpired is a decision to approve a request from an earlier month, whose allowance is no longer the teacher's.
	ErrRequestExpired = errors.New("ai: the request is from an earlier month")
)

// Scope says which budget a reservation ran into.
type Scope string

// The two budgets.
const (
	ScopeTeacher Scope = "teacher"
	ScopePool    Scope = "pool"
)

// Exhausted is the refusal of a reservation, and says whether the teacher's allowance or the centre's pool could not cover it. It satisfies errors.Is(err, ErrExhausted).
type Exhausted struct {
	Scope Scope
}

func (e *Exhausted) Error() string {
	return fmt.Sprintf("%s: %s", ErrExhausted, e.Scope)
}

func (e *Exhausted) Is(target error) bool { return target == ErrExhausted }
