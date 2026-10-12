package schedule

import "time"

// CloseOf returns the SQL expression for the moment an assignment stops taking
// attempts, over a row of app.assignments aliased a: its early close or the
// end of its window, whichever comes first. When override is not empty it is
// the alias of a row of per-student overrides, and that row's closes_at, when
// it is later, replaces the moment: an override never closes anyone sooner.
// The expression is NULL only for a row with neither date, which the table
// does not allow.
func CloseOf(override string) string {
	if override == "" {
		return `least(a.closed_at, a.closes_at)`
	}
	return `greatest(least(a.closed_at, a.closes_at), ` + override + `.closes_at)`
}

// Close is CloseOf in Go: the earlier of closedAt, when set, and closesAt,
// then the later of that and override, when set.
func Close(closesAt time.Time, closedAt, override *time.Time) time.Time {
	closes := closesAt
	if closedAt != nil && closedAt.Before(closes) {
		closes = *closedAt
	}
	if override != nil && override.After(closes) {
		closes = *override
	}
	return closes
}

// Override is what one student's override changes about an assignment's
// window. A nil field leaves that part as the assignment has it.
type Override struct {
	ClosesAt      *time.Time
	DurationMin   *int
	ExtraAttempts int
}

// OverrideSelect is the columns of a student's override as the LEFT JOIN
// OverrideJoin reads them, in the order OverrideColumns holds them.
const OverrideSelect = `o.closes_at, o.duration_minutes, o.extra_attempts`

// OverrideJoin is the LEFT JOIN, over a row of app.assignments aliased a, of
// the override of the student the SQL expression student names, aliased o. A
// student without an override reads NULL in every column of OverrideSelect.
func OverrideJoin(student string) string {
	return `LEFT JOIN app.assignment_student_overrides o ON o.assignment_id = a.id AND o.student_id = ` + student
}

// OverrideColumns is what a query scans OverrideSelect into. Every field is
// nil when the student has no override.
type OverrideColumns struct {
	ClosesAt      *time.Time
	DurationMin   *int
	ExtraAttempts *int
}

// Override returns the override the columns describe, or nil when the student
// has none.
func (c OverrideColumns) Override() *Override {
	if c.ExtraAttempts == nil {
		return nil
	}
	return &Override{ClosesAt: c.ClosesAt, DurationMin: c.DurationMin, ExtraAttempts: *c.ExtraAttempts}
}

// Window is the part of an assignment that decides when and how long a
// student may sit it.
type Window struct {
	OpensAt     time.Time
	ClosesAt    time.Time
	ClosedAt    *time.Time
	DurationMin int
	MaxAttempts int
}

// Close is the moment the window stops taking attempts: the earlier of its
// early close and its end.
func (w Window) Close() time.Time {
	return Close(w.ClosesAt, w.ClosedAt, nil)
}

// WithOverride returns the window one student sees: the later close, the
// duration an override sets and the attempts it adds. An override that closes
// no later than the window does leaves the close as it is. One that closes
// later replaces the window's end and lifts its early close, so the
// student's window reads as open until then. A nil override returns the
// window as it is.
func (w Window) WithOverride(o *Override) Window {
	if o == nil {
		return w
	}
	if o.ClosesAt != nil && o.ClosesAt.After(w.Close()) {
		w.ClosesAt, w.ClosedAt = *o.ClosesAt, nil
	}
	if o.DurationMin != nil {
		w.DurationMin = *o.DurationMin
	}
	w.MaxAttempts += o.ExtraAttempts
	return w
}
