package domain

import (
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"
	"time"
)

// ListInput selects a page of the assignments Scope reaches
// (visibility.AssignmentIDs), or every one under scope.all; a zero Scope
// matches nothing, and so does a ClassID of a class the scope does not teach
// unless EveryTarget is set.
type ListInput struct {
	Status *Status
	// ClassID narrows the list to assignments that target the class (G-12).
	ClassID *string
	Page    int
	Limit   int
	Scope   access.Scope
	// EveryTarget lifts the reach from the targets and leaves it on the rows,
	// for a caller who holds scope.all and lists their own: a row names and
	// counts all its classes and students, and ClassID matches any class.
	EveryTarget bool
}

// Request is the actor behind a write, for the audit row and for reach. A
// write touches only assignments ActorID reaches, and names only versions of
// tests ActorID owns, classes ActorID teaches and students ActorID reaches,
// unless All, the actor's scope.all, is set; anything else answers exactly as
// a missing id does.
type Request struct {
	ID        string
	ActorID   string
	All       bool
	IP        string
	UserAgent string
}

// Scope is the scope the request reads and writes under: ActorID's own, or
// everyone's with All.
func (r Request) Scope() access.Scope {
	return access.Scope{UserID: r.ActorID, All: r.All}
}

type WriteInput struct {
	TestVersionID string
	ClassIDs      []string
	StudentIDs    []string
	OpensAt       time.Time
	ClosesAt      time.Time
	DurationMin   int
	MaxAttempts   int
	ShuffleQ      bool
	ShuffleO      bool
	Review        Review
	Integrity     Integrity
	CloseNow      bool
	// Draft withholds it from students.
	Draft bool
	Now   time.Time
}

func (in WriteInput) Validate() error {
	var fields []FieldError

	if !in.ClosesAt.After(in.OpensAt) {
		fields = append(fields, FieldError{Field: "window.closesAt", Message: "Thời điểm đóng phải sau thời điểm mở."})
	}

	if !in.Draft && len(in.ClassIDs) == 0 && len(in.StudentIDs) == 0 {
		fields = append(fields, FieldError{Field: "targets", Message: "Chọn ít nhất một lớp hoặc một học viên."})
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

type (
	FieldError      = validation.Field
	ValidationError = validation.Error
)
