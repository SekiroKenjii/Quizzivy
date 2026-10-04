package domain

import "slices"

// Event is one switch of the notification settings, and one of the values
// notification_preferences_event_check allows.
type Event string

const (
	EventAttemptSubmitted  Event = "attempt.submitted"
	EventAttemptFlagged    Event = "attempt.flagged"
	EventAssignmentClosing Event = "assignment.closing"
	EventAssignmentDueSoon Event = "assignment.due_soon"
	EventResultReady       Event = "result.ready"
)

var events = []Event{
	EventAttemptSubmitted, EventAttemptFlagged, EventAssignmentClosing, EventAssignmentDueSoon, EventResultReady,
}

// Events returns every switch, in the order a set of preferences is answered.
func Events() []Event {
	return slices.Clone(events)
}

// Preference is one switch for one user: whether the notices its event
// governs are written in the app, and whether they will be sent by email
// once that channel exists.
type Preference struct {
	Event Event
	InApp bool
	Email bool
}

// DefaultPreference is the switch of a user who never saved it: in the app,
// and not by email.
func DefaultPreference(event Event) Preference {
	return Preference{Event: event, InApp: true}
}

// WithDefaults returns every switch in Events' order, taking each from stored
// when it is there and its default when it is not. A stored row whose event
// is not a switch is left out.
func WithDefaults(stored []Preference) []Preference {
	saved := make(map[Event]Preference, len(stored))
	for _, p := range stored {
		saved[p.Event] = p
	}
	out := make([]Preference, 0, len(events))
	for _, event := range events {
		if p, ok := saved[event]; ok {
			out = append(out, p)
			continue
		}
		out = append(out, DefaultPreference(event))
	}
	return out
}

// WholeSet returns given in Events' order when it holds every switch exactly
// once. It answers ErrUnknownEvent for an event that is not a switch, and
// ErrEventsNotOnceEach when one is repeated or missing.
func WholeSet(given []Preference) ([]Preference, error) {
	seen := make(map[Event]Preference, len(given))
	for _, p := range given {
		if !slices.Contains(events, p.Event) {
			return nil, ErrUnknownEvent
		}
		if _, twice := seen[p.Event]; twice {
			return nil, ErrEventsNotOnceEach
		}
		seen[p.Event] = p
	}
	if len(seen) != len(events) {
		return nil, ErrEventsNotOnceEach
	}
	out := make([]Preference, 0, len(events))
	for _, event := range events {
		out = append(out, seen[event])
	}
	return out, nil
}
