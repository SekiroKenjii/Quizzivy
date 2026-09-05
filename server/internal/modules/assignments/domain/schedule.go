package domain

import (
	"time"
)

// StatusAt is D-18's pure function: no scheduler, no stale row.
//
// The draft case does not weaken that. Publishing is an act by the teacher, not
// a timestamp arriving, so nothing has to flip a row when a clock passes -- the
// window rule reads exactly as it did once PublishedAtOf exists.
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

var Schedule ScheduleManager
