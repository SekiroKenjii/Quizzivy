// Package domain models a Word import: private sources and their revisions, fenced processing runs,
// the reviewable exam draft with its findings, and the plan that commits it as one draft test.
package domain

import (
	"errors"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/paging"
	"time"
)

var (
	ErrNotFound    = errors.New("imports: not found")
	ErrConflict    = errors.New("imports: revision, lifecycle or idempotency conflict")
	ErrQuota       = errors.New("imports: quota exceeded")
	ErrBusy        = errors.New("imports: intake busy")
	ErrTooLarge    = errors.New("imports: source limit exceeded")
	ErrUnsupported = errors.New("imports: unsupported source")
	ErrInvalid     = errors.New("imports: invalid source")
	ErrStale       = errors.New("imports: draft saved elsewhere")
	ErrNoDraft     = errors.New("imports: no draft yet")
	ErrBadDraft    = errors.New("imports: malformed draft edit")

	ErrProcessingOff = errors.New("imports: processing is not enabled on this server")
	ErrFilesRemoved  = errors.New("imports: retention removed this import's files")
)

const MaxSourceBytes int64 = 25 << 20

// Statuses in which a teacher may add or replace a source; a new source set returns the import to awaiting_sources.
func AcceptsSources(status string) bool {
	return status == "awaiting_sources" || status == "failed" || status == "needs_review"
}

type Import struct {
	ID, Title, Status, CreatedBy string
	Revision, SourceRevision     int64
	CreatedAt, UpdatedAt         time.Time
	Sources                      []Source
	PendingUploads               int
	Run                          *RunSummary
	DraftRevision                int64
	TestID                       *string
	FilesRemovedAt               *time.Time
	ClosedIdle                   bool
}

// RunSummary is the latest processing run as the teacher sees it.
type RunSummary struct {
	ID, Status, Stage    string
	Attempt, MaxAttempts int
	ErrorCode            *string
	Profile              RecognitionProfile
	CreatedAt, UpdatedAt time.Time
}

type Source struct {
	ID, ImportID, UploadID, Role, Filename, Format, StorageKey, UploadedBy string
	ExpectedRevision, Bytes, SourceRevision                                int64
	SHA256                                                                 []byte
	Ready                                                                  bool
	CreatedAt                                                              time.Time
}

type Create struct {
	RequestID, Title string
	Actor            actor.Actor
}
type Reserve struct {
	Source Source
	Actor  actor.Actor
}
type Finish struct {
	ImportID, SourceID string
	Actor              actor.Actor
}
type Receipt struct {
	Import Import
	Source Source
}
type Filter struct {
	Search, Status string
	Page, Limit    int
}
type List struct {
	Items []Import
	Page  paging.Page
}

// Quotas bounds retained reservations as well as completed sources; failed storage writes do not evade accounting.
type Quotas struct {
	ActorImports, GlobalImports, SourcesPerImport int
	ActorBytes, GlobalBytes                       int64
}

// DefaultQuotas returns conservative development limits, pending production capacity approval.
func DefaultQuotas() Quotas {
	return Quotas{ActorImports: 10, GlobalImports: 100, SourcesPerImport: 32, ActorBytes: 256 << 20, GlobalBytes: 1 << 30}
}
