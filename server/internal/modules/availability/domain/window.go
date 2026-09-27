// Package domain is the availability model: the maintenance windows during which the API is closed, as the operator scheduled them, and what the API tells anyone who asks about them.
package domain

import (
	"context"
	"time"
)

// Window is a scheduled maintenance window, from StartsAt until EndsAt.
type Window struct {
	StartsAt time.Time
	EndsAt   time.Time
}

// ActiveAt reports whether the window covers the instant now.
func (w Window) ActiveAt(now time.Time) bool {
	return !now.Before(w.StartsAt) && now.Before(w.EndsAt)
}

// Status is the next window that has not ended, if there is one, and whether
// it is under way.
type Status struct {
	Window *Window
	Active bool
}

// Repository reads windows. The API may read them and nothing else.
type Repository interface {
	// Next returns the earliest window that was not cancelled and ends after
	// now, or nil when there is none.
	Next(ctx context.Context, now time.Time) (*Window, error)
}
