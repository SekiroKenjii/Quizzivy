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

// GroupMutation identifies the aggregate revision a writer observed; section groups also require the enclosing test revision.
type GroupMutation struct {
	ID                    string
	ExpectedRevision      int64
	ExpectedTestUpdatedAt time.Time
	ActorID               string
	Now                   time.Time
	IP                    string
	UserAgent             string
}

// UpdateGroupInput replaces the complete editable graph without changing its owner.
type UpdateGroupInput struct {
	GroupMutation
	Bundle GroupBundle
}

// CopyGroupInput copies one observed source revision into an independent bank or section graph.
type CopyGroupInput struct {
	SourceID               string
	ExpectedSourceRevision int64
	OwnerSectionID         *string
	ExpectedTestUpdatedAt  time.Time
	ActorID                string
	Now                    time.Time
	IP                     string
	UserAgent              string
}
