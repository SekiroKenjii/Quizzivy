package domain

import "errors"

var (
	// ErrNoRecipient is a notice or a request that names no user.
	ErrNoRecipient = errors.New("notifications: no recipient")
	// ErrUnknownKind is a notice whose kind this binary does not word.
	ErrUnknownKind = errors.New("notifications: unknown kind")
	// ErrParamsMismatch is a notice without params, or with another kind's.
	ErrParamsMismatch = errors.New("notifications: the params are not this kind's")
	// ErrInvalidParams is a param outside the bounds the contract gives it.
	ErrInvalidParams = errors.New("notifications: a param is out of bounds")
	// ErrInvalidTarget is a target with an unknown route, or without the id
	// its route cannot be opened without.
	ErrInvalidTarget = errors.New("notifications: the target cannot be opened")
	// ErrInvalidDedupeKey is a dedupe key that is empty or longer than
	// MaxDedupeKey characters.
	ErrInvalidDedupeKey = errors.New("notifications: the dedupe key is empty or too long")
	// ErrUnknownMerge is a notice that does not say how it merges.
	ErrUnknownMerge = errors.New("notifications: unknown merge mode")
	// ErrNoIDs is a request to mark notifications read by id that names none.
	ErrNoIDs = errors.New("notifications: no id to mark read")
	// ErrTooManyIDs is a request to mark more than MaxMarkedIDs read at once.
	ErrTooManyIDs = errors.New("notifications: too many ids to mark read")
	// ErrUnknownEvent is a preference for an event that is not a switch.
	ErrUnknownEvent = errors.New("notifications: unknown event")
	// ErrEventsNotOnceEach is a set of preferences that does not hold every
	// event exactly once.
	ErrEventsNotOnceEach = errors.New("notifications: every event must appear exactly once")
)
