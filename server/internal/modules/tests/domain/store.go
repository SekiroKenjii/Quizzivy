package domain

import (
	"time"
)

// CreateInput is a new empty draft.
type CreateInput struct {
	Title       string
	Description *string
	ActorID     string
	Now         time.Time
	IP          string
	UserAgent   string
}
