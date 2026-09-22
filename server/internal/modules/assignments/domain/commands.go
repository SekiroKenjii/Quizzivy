package domain

import (
	"quizzivy/internal/shared/validation"
	"time"
)

type ListInput struct {
	Status *Status
	// ClassID narrows the list to assignments that target the class (G-12).
	ClassID *string
	Page    int
	Limit   int
}

// Request is the actor behind a write, for the audit row.
type Request struct {
	ID        string
	ActorID   string
	IP        string
	UserAgent string
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
