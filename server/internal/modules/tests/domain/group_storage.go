package domain

import "time"

// StoredGroup is an independent context graph with its owner and aggregate revision.
type StoredGroup struct {
	Bundle         GroupBundle
	OwnerSectionID *string
	Revision       int64
	ArchivedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CreateGroupInput materializes a validated graph atomically; test-owned groups require the enclosing draft revision.
type CreateGroupInput struct {
	Bundle                GroupBundle
	OwnerSectionID        *string
	ExpectedTestUpdatedAt time.Time
	ActorID               string
	Now                   time.Time
	IP                    string
	UserAgent             string
}
