package domain

import "github.com/google/uuid"

// Route is the page a notification opens. The client resolves it, so it is
// one of the contract's NotificationTarget routes and never a URL.
type Route string

const (
	RouteAssignment        Route = "assignment"
	RouteAttempt           Route = "attempt"
	RouteGrading           Route = "grading"
	RouteClasses           Route = "classes"
	RouteResult            Route = "result"
	RouteStudentAssignment Route = "studentAssignment"
)

type requirement struct {
	assignment bool
	attempt    bool
}

var routes = map[Route]requirement{
	RouteAssignment:        {assignment: true},
	RouteAttempt:           {attempt: true},
	RouteGrading:           {},
	RouteClasses:           {},
	RouteResult:            {attempt: true},
	RouteStudentAssignment: {assignment: true},
}

// Target is where a click on a notification goes, stored as the contract's
// NotificationTarget. RouteAssignment and RouteStudentAssignment need
// AssignmentID, RouteAttempt and RouteResult need AttemptID, and either id may
// accompany any route.
type Target struct {
	Route        Route  `json:"route"`
	AssignmentID string `json:"assignmentId,omitempty"`
	AttemptID    string `json:"attemptId,omitempty"`
}

// Validate answers ErrInvalidTarget for an unknown route, for an id that is
// not a uuid, and for a route without the id it needs.
func (t Target) Validate() error {
	needs, known := routes[t.Route]
	if !known {
		return ErrInvalidTarget
	}
	if !idOrNone(t.AssignmentID) || !idOrNone(t.AttemptID) {
		return ErrInvalidTarget
	}
	if needs.assignment && t.AssignmentID == "" || needs.attempt && t.AttemptID == "" {
		return ErrInvalidTarget
	}
	return nil
}

func idOrNone(id string) bool {
	if id == "" {
		return true
	}
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}
