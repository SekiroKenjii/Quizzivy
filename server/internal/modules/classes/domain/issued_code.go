package domain

import (
	"errors"
	"time"
)

var ErrClassNotFound = errors.New("join: class not found")

// IssuedCode is the metadata of an active code. It never carries the plaintext:
// that exists only in the response to the request that created it (§13.3).
type IssuedCode struct {
	ID        string
	ClassID   string
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

type RotateInput struct {
	ClassID     string
	ActorUserID string
	CodeHash    []byte
	Hint        string
	ExpiresAt   time.Time
	MaxUses     *int
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type RevokeInput struct {
	ClassID     string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}
