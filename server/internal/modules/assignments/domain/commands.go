package domain

import (
	"strings"
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

type FieldError struct{ Field, Message string }

type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "assignments: " + strings.Join(parts, "; ")
}

func (in WriteInput) Validate() error {
	var fields []FieldError

	if !in.ClosesAt.After(in.OpensAt) {
		fields = append(fields, FieldError{"window.closesAt", "Thời điểm đóng phải sau thời điểm mở."})
	}

	if !in.Draft && len(in.ClassIDs) == 0 && len(in.StudentIDs) == 0 {
		fields = append(fields, FieldError{"targets", "Chọn ít nhất một lớp hoặc một học viên."})
	}

	// Remove together with the auto_submit implementation (T-5.1).
	if in.Integrity.OnLimitExceeded == "auto_submit" {
		fields = append(fields, FieldError{
			"integrity.onLimitExceeded",
			"Chế độ tự động nộp bài chưa khả dụng.",
		})
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}
