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

// Window is the part of an assignment that decides when and how long a
// student may sit it.
type Window struct {
	OpensAt     time.Time
	ClosesAt    time.Time
	DurationMin int
	MaxAttempts int
}

// WithOverride returns the window one student sees: the later close, the
// duration an override sets and the attempts it adds. A nil override returns
// the window as it is.
func (w Window) WithOverride(o *Override) Window {
	if o == nil {
		return w
	}
	w.ClosesAt = Close(w.ClosesAt, nil, o.ClosesAt)
	if o.DurationMin != nil {
		w.DurationMin = *o.DurationMin
	}
	w.MaxAttempts += o.ExtraAttempts
	return w
}
