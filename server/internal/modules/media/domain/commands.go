package domain

import (
	"quizzivy/internal/shared/access"
	"time"
)

// ListInput selects a page of the library. Scope limits it to the caller's own
// assets, or every asset under scope.all; a zero Scope matches nothing.
type ListInput struct {
	Kind  *Kind
	Page  int
	Limit int
	Scope access.Scope
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
	Now              time.Time
	IP               *string
	UserAgent        *string
}
