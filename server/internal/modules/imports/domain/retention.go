package domain

import "time"

// Retention is how long an import keeps its original files and review draft:
// AfterCommit once its test exists, AfterCancel once cancelled. An import
// untouched for Idle while waiting on a teacher is closed and its files are
// removed at once. History rows outlive all three.
type Retention struct {
	AfterCommit, AfterCancel, Idle time.Duration
}

const day = 24 * time.Hour

// DefaultRetention is the policy Thuong approved for D-08 on 2026-09-25.
func DefaultRetention() Retention {
	return Retention{AfterCommit: 30 * day, AfterCancel: 7 * day, Idle: 60 * day}
}

// Days reports each period in whole days, as teachers are told them.
func (r Retention) Days() (afterCommit, afterCancel, idle int) {
	return int(r.AfterCommit / day), int(r.AfterCancel / day), int(r.Idle / day)
}

// Cursor is an import's place in the removal sweep's (updated_at, id) order;
// the zero value starts before every import.
type Cursor struct {
	UpdatedAt time.Time
	ID        string
}
