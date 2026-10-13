package domain

import "slices"

// Audience is the console that reads a notification: the teacher's bell or
// the student's. The empty Audience names neither and means every kind, which
// is what a caller that does not say gets.
type Audience string

const (
	AudienceTeacher Audience = "teacher"
	AudienceStudent Audience = "student"
)

var kindsByAudience = map[Audience][]Kind{
	AudienceTeacher: {AttemptSubmitted, AttemptFlagged, AssignmentClosing, ClassJoined, JoinCodesRotated, AIAllowanceWarning, AICreditRequest, AICreditRequestDecided},
	AudienceStudent: {AssignmentOpened, AssignmentDueSoon, AssignmentExtended, ResultReady},
}

// Valid reports whether a is one of the two consoles or empty.
func (a Audience) Valid() bool {
	_, ok := kindsByAudience[a]
	return ok || a == ""
}

// KindsOf returns the kinds a's bell shows, in the contract's order, and nil
// for the empty Audience, which restricts nothing. Every kind belongs to
// exactly one Audience; the contract's table says which, and a test holds the
// two to each other.
func KindsOf(a Audience) []Kind {
	return slices.Clone(kindsByAudience[a])
}

// Audience is the console that reads k, and the empty Audience for a kind
// this binary does not know.
func (k Kind) Audience() Audience {
	for audience, kinds := range kindsByAudience {
		if slices.Contains(kinds, k) {
			return audience
		}
	}
	return ""
}
