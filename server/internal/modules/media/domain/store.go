package domain

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("media: asset not found")

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
