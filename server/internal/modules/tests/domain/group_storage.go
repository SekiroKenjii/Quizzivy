package domain

import (
	"context"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
	"time"
)

// GroupRepository persists complete independent graphs and their revision-guarded lifecycle.
type GroupRepository interface {
	List(context.Context, GroupListInput) ([]GroupSummary, paging.Page, error)
	Get(context.Context, access.Scope, string) (StoredGroup, error)
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
	Scope  access.Scope
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

// StoredGroup is an independent context graph with its owner and aggregate
// revision. OwnerID is whose it is: the group's own owner in the bank, its
// test's owner in a section, whatever the group row records.
type StoredGroup struct {
	Bundle         GroupBundle
	OwnerSectionID *string
	OwnerID        string
	Revision       int64
	ArchivedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	TestUpdatedAt  *time.Time
}

// CreateGroupInput materializes a validated graph atomically; test-owned
// groups require the enclosing draft revision. A section destination outside
// Scope is ErrNotFound, and Grants must hold the key RequireGroupWrite names
// for the destination.
type CreateGroupInput struct {
	Bundle                GroupBundle
	OwnerSectionID        *string
	ExpectedTestUpdatedAt time.Time
	ActorID               string
	Now                   time.Time
	IP                    string
	UserAgent             string
	Scope                 access.Scope
	Grants                access.Set
}

// GroupMutation identifies the aggregate revision a writer observed; section
// groups also require the enclosing test revision. A group outside Scope is
// ErrNotFound, and Grants must hold the key RequireGroupWrite names for the
// stored group.
type GroupMutation struct {
	ID                    string
	ExpectedRevision      int64
	ExpectedTestUpdatedAt time.Time
	ActorID               string
	Now                   time.Time
	IP                    string
	UserAgent             string
	Scope                 access.Scope
	Grants                access.Set
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
	Scope                  access.Scope
	Grants                 access.Set
}

// RequireGroupWrite narrows the any-of permission the group writes declare
// to the key their target needs: content.questions.write for a bank group,
// content.tests.write for one a test section owns. It returns ErrForbidden
// when grants lack it.
func RequireGroupWrite(grants access.Set, bank bool) error {
	key := access.ContentTestsWrite
	if bank {
		key = access.ContentQuestionsWrite
	}
	if !grants.Has(key) {
		return ErrForbidden
	}
	return nil
}
