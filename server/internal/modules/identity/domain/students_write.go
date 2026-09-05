package domain

import (
	"time"
)

// WriteRequest is the admin behind a write, for the audit row.
type WriteRequest struct {
	ActorID   string
	IP        string
	UserAgent string
}

type NewStudent struct {
	Email    string
	FullName string
	ClassIDs []string
	// Hash is computed by the caller: Argon2id blocks on a four-slot semaphore
	// that must not be held across a transaction.
	Hash string
	Now  time.Time
}

type StudentPatch struct {
	ID string
	// nil means the caller did not send the field.
	FullName *string
	Email    *string
	Disabled *bool
	Now      time.Time
}
