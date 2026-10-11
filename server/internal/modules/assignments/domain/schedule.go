package domain

import (
	"time"
)

// StatusAt is D-18's pure function: no scheduler, no stale row.
func (ScheduleManager) StatusAt(now time.Time, PublishedAtOf *time.Time, opensAt, closesAt time.Time, ClosedAtOf *time.Time) Status {
	if PublishedAtOf == nil {
		return Draft
	}
	if ClosedAtOf != nil && !now.Before(*ClosedAtOf) {
		return Closed
	}
	switch {
	case now.Before(opensAt):
		return Scheduled
	case now.Before(closesAt):
		return Open
	default:
		return Closed
	}
}

// ScheduleManager derives an assignment's status and publication moments from its window.
type ScheduleManager struct{}

// NextPublishedAt keeps an already-published assignment published. Saving one
// with draft:true again does not un-give it -- students may already be sitting
// it, and the only way back out is closing it.
func (ScheduleManager) NextPublishedAt(current *time.Time, in WriteInput) *time.Time {
	if current != nil {
		return current
	}
	return Schedule.PublishedAtOf(in)
}

// PublishedAtOf is set once and never cleared: an assignment students have
// already been given cannot be pulled back into a draft, only closed.
func (ScheduleManager) PublishedAtOf(in WriteInput) *time.Time {
	if in.Draft {
		return nil
	}
	return &in.Now
}

func (ScheduleManager) ClosedAtOf(in WriteInput) *time.Time {
	if in.CloseNow {
		return &in.Now
	}
	return nil
}

// ClosedAtAfter is the early close an assignment holds once in is saved over
// it: the one it has, which an update never lifts, or in.Now when in closes it
// now.
func (ScheduleManager) ClosedAtAfter(current *time.Time, in WriteInput) *time.Time {
	if current == nil && in.CloseNow {
		return &in.Now
	}
	return current
}

// Reopens reports whether saving in over an assignment turns one that is
// closed at in.Now into one that is not: a window that ends ahead of in.Now,
// or starts ahead of it, or starts after the early close the assignment keeps
// (the schema refuses such a pair). A window that leaves it closed does not
// reopen it, whether it is the stored one or an earlier close, and neither
// does a later close once the assignment is closed early.
func (m ScheduleManager) Reopens(publishedAt, closedAt *time.Time, opensAt, closesAt time.Time, in WriteInput) bool {
	if m.StatusAt(in.Now, publishedAt, opensAt, closesAt, closedAt) != Closed {
		return false
	}
	after := m.ClosedAtAfter(closedAt, in)
	if after != nil && in.OpensAt.After(*after) {
		return true
	}
	return m.StatusAt(in.Now, publishedAt, in.OpensAt, in.ClosesAt, after) != Closed
}

var Schedule ScheduleManager
