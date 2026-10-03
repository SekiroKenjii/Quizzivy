package domain

import (
	"context"
	"encoding/json"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"time"
)

// Outcome is a finished run: the private result envelope and the machine draft it produced.
type Outcome struct {
	Result json.RawMessage
	Draft  json.RawMessage
}

// StoredDraft is the current review copy of an import. Reprocessed means a newer machine draft
// arrived after teacher edits and was kept aside rather than overwriting them. CreatedBy is the
// import's creator, who owns whatever a commit of it creates.
type StoredDraft struct {
	ImportID, Title, Status string
	CreatedBy               string
	Revision                int64
	Draft                   Draft
	Reprocessed             bool
	UpdatedAt               time.Time
}

type SaveDraft struct {
	ImportID         string
	ExpectedRevision int64
	Draft            Draft
	Actor            actor.Actor
}

// Commit records which draft revision became which test; one per import.
type Commit struct {
	ImportID, RequestID string
	DraftRevision       int64
	Digest              []byte
	TestID              *string
}

type CommitRecord struct {
	Commit
	Actor actor.Actor
}

// ReviewState is a stored draft with the findings derived from it.
type ReviewState struct {
	Draft  StoredDraft
	Review Review
}

// Drafts reads and saves review copies under revision control; AdoptCandidate replaces an edited draft
// with the machine draft kept aside, and fails with ErrConflict when there is none. Draft and Commit reach
// the imports a scope's user created, or every one under scope.all; SaveDraft and AdoptCandidate reach an
// import their Actor.ID created, or any under Actor.Scope.All. Another creator's import answers ErrNotFound
// exactly as a missing one, never ErrNoDraft, ErrConflict or ErrStale.
type Drafts interface {
	Draft(context.Context, access.Scope, string) (StoredDraft, error)
	SaveDraft(context.Context, SaveDraft) (StoredDraft, error)
	AdoptCandidate(context.Context, AdoptCandidate) (StoredDraft, error)
	Commit(context.Context, access.Scope, string) (Commit, error)
}

type AdoptCandidate struct {
	ImportID         string
	ExpectedRevision int64
	Actor            actor.Actor
}

// CommitStore closes an import inside the transaction that created its test; it fails with
// ErrConflict when the import is no longer under review or ErrStale when the draft moved on.
type CommitStore interface {
	RecordCommit(context.Context, CommitRecord) error
}
