package domain

import (
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"
	"time"
	"unicode/utf8"
)

// ListInput selects a page of the assignments Scope reaches
// (visibility.AssignmentIDs), or every one under scope.all; a zero Scope
// matches nothing, and so does a class of ClassIDs the scope does not teach
// unless EveryTarget is set.
type ListInput struct {
	Status *Status
	// ClassIDs narrows the list to assignments that target any of the classes (G-12).
	ClassIDs []string
	// Query keeps the assignments whose test title, or the name of a target
	// class the scope reaches, contains it, ignoring case and accents.
	Query string
	Page  int
	Limit int
	Scope access.Scope
	// EveryTarget lifts the reach from the targets and leaves it on the rows,
	// for a caller who holds scope.all and lists their own: a row names and
	// counts all its classes and students, and ClassIDs match any class.
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
	// ReleaseSet, ClassAverageSet and StudentNoteSet say that the request
	// named the field. A create takes Review and StudentNote as they stand; an
	// update keeps the stored value of a field it did not name.
	ReleaseSet      bool
	ClassAverageSet bool
	StudentNote     *string
	StudentNoteSet  bool
	// Draft withholds it from students.
	Draft bool
	Now   time.Time
}

func (in WriteInput) Validate() error {
	var fields []FieldError

	if !in.ClosesAt.After(in.OpensAt) {
		fields = append(fields, FieldError{Field: "window.closesAt", Message: "Thời điểm đóng phải sau thời điểm mở."})
	}

	if !in.Review.Release.Valid() {
		fields = append(fields, FieldError{Field: "review.release", Message: "Thời điểm công bố kết quả không hợp lệ."})
	}

	if in.StudentNote != nil && utf8.RuneCountInString(*in.StudentNote) > MaxStudentNote {
		fields = append(fields, FieldError{Field: "studentNote", Message: "Ghi chú cho học viên tối đa 500 ký tự."})
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
