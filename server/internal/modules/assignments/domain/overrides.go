package domain

import (
	"strings"
	"time"
)

// StudentOverride is what a teacher changed about one student's turn at an
// assignment: a later close, a longer time limit, more attempts, and why. A
// nil ClosesAt or DurationMin leaves that part as the assignment has it. It is
// the teacher's record and never part of anything a student receives.
type StudentOverride struct {
	StudentID     string
	StudentName   string
	ClosesAt      *time.Time
	DurationMin   *int
	ExtraAttempts int
	Reason        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OverrideInput asks to give StudentIDs an override. A field that is set
// replaces the stored one; a field that is nil keeps it. ExtendBy, in minutes,
// moves each student's own close later from where it is now and is the
// alternative to ClosesAt. Notify is recorded with the audit entry, and Now is
// the moment the audit entries carry.
type OverrideInput struct {
	StudentIDs    []string
	ExtendBy      *int
	ClosesAt      *time.Time
	DurationMin   *int
	ExtraAttempts *int
	Reason        string
	Notify        bool
	Now           time.Time
}

// CleanReason is the reason as it is stored: trimmed.
func (in OverrideInput) CleanReason() string {
	return strings.TrimSpace(in.Reason)
}

// Validate refuses what the schema cannot: a reason of only whitespace, both
// ways of naming the close at once, and a request that changes nothing. That
// ClosesAt is ahead is judged where the write is, at the database's clock.
func (in OverrideInput) Validate() error {
	var fields []FieldError

	if in.CleanReason() == "" {
		fields = append(fields, FieldError{Field: "reason", Message: "Hãy ghi lý do."})
	}
	if in.ExtendBy != nil && in.ClosesAt != nil {
		fields = append(fields, FieldError{Field: "closesAt", Message: "Chỉ chọn một trong hai: gia hạn thêm hoặc thời điểm đóng mới."})
	}
	if in.ExtendBy == nil && in.ClosesAt == nil && in.DurationMin == nil && in.ExtraAttempts == nil {
		fields = append(fields, FieldError{Field: "extendBy", Message: "Hãy chọn ít nhất một thay đổi: thời gian đóng, thời lượng hoặc số lượt làm."})
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}
