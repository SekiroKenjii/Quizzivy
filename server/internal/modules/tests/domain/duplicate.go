package domain

import (
	"time"
)

// DuplicateInput copies a test's draft structure.
type DuplicateInput struct {
	ID        string
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}
