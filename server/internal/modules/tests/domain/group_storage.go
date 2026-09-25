package domain

import (
	"context"
	"quizzivy/internal/shared/paging"
	"time"
)

// GroupRepository persists complete independent graphs and their revision-guarded lifecycle.
type GroupRepository interface {
	List(context.Context, GroupListInput) ([]GroupSummary, paging.Page, error)
	Get(context.Context, string) (StoredGroup, error)
	Create(context.Context, CreateGroupInput) (StoredGroup, error)
	Update(context.Context, UpdateGroupInput) (StoredGroup, error)
	Copy(context.Context, CopyGroupInput) (StoredGroup, error)
	SetArchived(context.Context, GroupMutation, bool) (StoredGroup, error)
	Delete(context.Context, GroupMutation) error
	RemoveFromSection(context.Context, GroupMutation) error
}

// GroupListInput selects bank-owned groups; section copies are accessed through their enclosing test.
type GroupListInput struct {
	Query  string
	Tag    string
	Status string
	Page   int
	Limit  int
}

// GroupSummary is a bounded bank listing without material text, answer keys or transcripts.
type GroupSummary struct {
	ID             string
	Title          string
	Revision       int64
	QuestionCount  int
	RecordingCount int
	TotalPoints    string
	Tags           []string
	ArchivedAt     *time.Time
	UpdatedAt      time.Time
}

// StoredGroup is an independent context graph with its owner and aggregate revision.
type StoredGroup struct {
	Bundle         GroupBundle
	OwnerSectionID *string
	Revision       int64
	ArchivedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	TestUpdatedAt  *time.Time
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
