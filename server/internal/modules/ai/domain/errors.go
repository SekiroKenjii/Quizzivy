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

var (
	// ErrOff is an operation asked of an assistant that has no provider configured.
	ErrOff = errors.New("ai: the assistant is not configured")
	// ErrUnavailable is a call that produced nothing a teacher can use: the provider failed after its retries, refused, or answered something that fails the operation's own checks. What the provider billed was settled; the rest was released.
	ErrUnavailable = errors.New("ai: the provider gave no usable answer")
	// ErrReadOff is an operation that would send a student's answer, asked by a teacher who has turned that off.
	ErrReadOff = errors.New("ai: the teacher does not let the assistant read student answers")
	// ErrForbidden is a write to a teacher the actor may not act on under the subset rule.
	ErrForbidden = errors.New("ai: the actor may not act on this teacher")
	// ErrMissingPermission is an operation that needs a second permission the actor lacks: marking by hand, for a suggestion that reads a student's answer.
	ErrMissingPermission = errors.New("ai: the actor lacks a permission the operation needs")
	// ErrNotFound is a test, passage, question, attempt or assignment that does not exist or that the caller does not reach: the two answer alike.
	ErrNotFound = errors.New("ai: no such item in the caller's reach")
	// ErrNothingToRead is an operation whose material is empty: a draft with no question, a bank with none of the caller's own, an assignment nobody handed in.
	ErrNothingToRead = errors.New("ai: there is nothing to read")
	// ErrTooLarge is an operation whose material is more than one call reads.
	ErrTooLarge = errors.New("ai: there is too much to read at once")
	// ErrNotChoice is a request for wrong options on a question that is not a single or multiple choice question.
	ErrNotChoice = errors.New("ai: the question is not a choice question")
	// ErrNoKey is a question with no answer key to explain.
	ErrNoKey = errors.New("ai: the question has no answer key")
	// ErrNoSample is an essay with no sample answer to explain from.
	ErrNoSample = errors.New("ai: the essay has no sample answer")
	// ErrNotManual is a score suggestion for a question that is not marked by hand.
	ErrNotManual = errors.New("ai: the question is not marked by hand")
	// ErrUnanswered is a score suggestion for an answer the student left empty.
	ErrUnanswered = errors.New("ai: the student left the question unanswered")
	// ErrAttemptInProgress is a score suggestion for a paper not yet handed in.
	ErrAttemptInProgress = errors.New("ai: the attempt is still in progress")
	// ErrAttemptVoided is a score suggestion for a paper that does not count.
	ErrAttemptVoided = errors.New("ai: the attempt is voided")
)
