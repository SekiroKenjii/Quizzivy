// Package domain is the notifications context: a Notification tells one
// user that something happened, as a Kind and the Params its sentence is
// built from; a Notice is one a producer hands in, merged into the row its
// user already holds under the same dedupe key; a Preference is one switch
// that says whether the notices an Event governs are written at all.
package domain

import (
	"encoding/json"
	"time"
)

// Kind is what a notification is about. The reader's console words the item
// from it, so it is one of the contract's NotificationKind values and
// nothing else.
type Kind string

const (
	AttemptSubmitted   Kind = "attempt.submitted"
	AttemptFlagged     Kind = "attempt.flagged"
	AssignmentClosing  Kind = "assignment.closing"
	ClassJoined        Kind = "class.joined"
	JoinCodesRotated   Kind = "join_codes.rotated"
	AssignmentOpened   Kind = "assignment.opened"
	AssignmentDueSoon  Kind = "assignment.due_soon"
	AssignmentExtended Kind = "assignment.extended"
	ResultReady        Kind = "result.ready"
)

var governing = map[Kind]Event{
	AttemptSubmitted:   EventAttemptSubmitted,
	AttemptFlagged:     EventAttemptFlagged,
	AssignmentClosing:  EventAssignmentClosing,
	AssignmentOpened:   EventAssignmentDueSoon,
	AssignmentDueSoon:  EventAssignmentDueSoon,
	AssignmentExtended: EventAssignmentDueSoon,
	ResultReady:        EventResultReady,
}

var switchless = map[Kind]bool{ClassJoined: true, JoinCodesRotated: true}

// Known reports whether k is a kind this binary writes and words.
func (k Kind) Known() bool {
	_, governed := governing[k]
	return governed || switchless[k]
}

// Event is the switch that governs k, and false for a kind that has none and
// is therefore always written.
func (k Kind) Event() (Event, bool) {
	event, ok := governing[k]
	return event, ok
}

// Notification is one stored notification as its reader lists it. Params is
// the stored object, whose names are the contract's NotificationParams.
type Notification struct {
	ID        string
	Kind      Kind
	Params    json.RawMessage
	Target    *Target
	CreatedAt time.Time
	ReadAt    *time.Time
}

const (
	// DefaultLimit is the page a list answers when the caller names no size.
	DefaultLimit = 20
	// MaxLimit is the largest page a list answers.
	MaxLimit = 50
	// MaxMarkedIDs is how many notifications one call may mark read by id.
	MaxMarkedIDs = 100
	// Retention is how long a notification is kept, counted from when it was
	// first written; a merge does not extend it.
	Retention = 180 * 24 * time.Hour
)

// ListQuery asks for one user's notifications older than Before, newest
// first. Before is a position in that user's own list: an id the user does
// not hold only bounds the page. An empty Before starts at the newest.
type ListQuery struct {
	UserID string
	Before string
	Limit  int
}

// Size is the page the query gets: Limit, DefaultLimit when Limit is zero or
// less, and never more than MaxLimit.
func (q ListQuery) Size() int {
	if q.Limit <= 0 {
		return DefaultLimit
	}
	return min(q.Limit, MaxLimit)
}

// Page is one page of a user's notifications. NextBefore is the id of the
// last item when older ones exist, and empty on the last page.
type Page struct {
	Items      []Notification
	NextBefore string
}

// Summary is what the reader's shell shows before any page is open.
type Summary struct {
	UnreadNotifications int
}
