package domain

import (
	"time"
)

// UpdateRequest is one autosave.
type UpdateRequest struct {
	ID        string
	Input     UpdateInput
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}
