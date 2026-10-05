package domain

import (
	"quizzivy/internal/shared/access"
	"time"
)

// ListInput selects a page of the library. Scope limits it to the caller's own
// assets, or every asset under scope.all; a zero Scope matches nothing. Query
// matches the display name and the original filename without accents or case.
// Unused keeps the assets no live question uses.
type ListInput struct {
	Kind   *Kind
	Query  string
	Unused bool
	Page   int
	Limit  int
	Scope  access.Scope
}

// DeleteInput is one soft delete, with the audit context it must record. The
// asset must belong to ActorID unless All, the actor's scope.all, is set; any
// other asset answers ErrNotFound, exactly as a missing one does.
type DeleteInput struct {
	ID        string
	ActorID   string
	All       bool
	Now       time.Time
	IP        string
	UserAgent string
}

// InsertInput is one new asset row. OwnerID is whose library it joins; empty
// means the uploader. QuotaBytes is the most that library may hold once the
// row is in it.
type InsertInput struct {
	ID               string
	Kind             Kind
	StorageKey       string
	MimeType         string
	Bytes            int64
	DurationMs       *int
	OriginalFilename string
	ChecksumSHA256   []byte
	UploaderID       string
	OwnerID          string
	DisplayName      *string
	DefaultMaxPlays  *int
	Width            *int
	Height           *int
	QuotaBytes       int64
	Now              time.Time
	IP               *string
	UserAgent        *string
}

// UpdateInput is one change to what the library shows for an asset, with the
// audit context it must record. A nil DisplayName keeps the stored name.
// DefaultMaxPlays is written only when SetDefaultMaxPlays is set, and a nil
// one then clears the limit. The asset must be in the library and belong to
// ActorID unless All, the actor's scope.all, is set; any other asset answers
// ErrNotFound, exactly as a missing one does.
type UpdateInput struct {
	ID                 string
	ActorID            string
	All                bool
	DisplayName        *string
	SetDefaultMaxPlays bool
	DefaultMaxPlays    *int
	Now                time.Time
	IP                 string
	UserAgent          string
}

// ReplacementTarget is a scoped live library asset and its owner.
type ReplacementTarget struct {
	Asset   Asset
	OwnerID string
}

// ReplacementCounts names distinct question rows and groups affected by replacement.
type ReplacementCounts struct {
	Questions int
	Groups    int
}

// ReplaceResult is a committed replacement and its actual reference counts.
type ReplaceResult struct {
	Asset     Asset
	Repointed ReplacementCounts
	Left      ReplacementCounts
}

// ReplaceInput replaces one library asset through an exclusively owned pool transaction.
type ReplaceInput struct {
	ID    string
	Scope access.Scope
	Asset InsertInput
}
