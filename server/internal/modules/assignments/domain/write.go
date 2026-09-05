package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound         = errors.New("assignments: not found")
	ErrTestNotPublished = errors.New("assignments: test version is not published")
	ErrVersionLocked    = errors.New("assignments: attempts exist")
)

type FieldError struct{ Field, Message string }

type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "assignments: " + strings.Join(parts, "; ")
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

func Validate(in WriteInput) error {
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

// NextPublishedAt keeps an already-published assignment published. Saving one
// with draft:true again does not un-give it -- students may already be sitting
// it, and the only way back out is closing it.
func NextPublishedAt(current *time.Time, in WriteInput) *time.Time {
	if current != nil {
		return current
	}
	return PublishedAtOf(in)
}

// PublishedAtOf is set once and never cleared: an assignment students have
// already been given cannot be pulled back into a draft, only closed.
func PublishedAtOf(in WriteInput) *time.Time {
	if in.Draft {
		return nil
	}
	return &in.Now
}

func ClosedAtOf(in WriteInput) *time.Time {
	if in.CloseNow {
		return &in.Now
	}
	return nil
}
