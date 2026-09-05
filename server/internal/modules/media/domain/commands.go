package domain

import (
	"time"
)

// ListInput selects a page of the library.
type ListInput struct {
	Kind  *Kind
	Page  int // 1-based; below 1 reads as the first
	Limit int // clamped to [1, MaxLimit]
}

// DeleteInput is one soft delete, with the audit context it must record.
type DeleteInput struct {
	ID        string
	ActorID   string
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
