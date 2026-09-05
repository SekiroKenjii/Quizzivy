package domain

import (
	"time"
)

// ListInput selects a page of classes. Query matches the name, accent-folded
// on both sides like every other search here (D-11).
type ListInput struct {
	Query string
	Page  int
	Limit int
	// One of active (the default), joinable, archived, all.
	Status string
}

// MembersInput selects a page of one class's roster. Query matches name or
// email.
type MembersInput struct {
	Query string
	Page  int
	Limit int
}

// UpdateInput carries only the fields the caller actually sent, so a PATCH that
// renames a class cannot silently clear its description.
type UpdateInput struct {
	Name *string
	// nil means "the caller did not send it".
	Description     *string
	SelfJoinEnabled *bool
}

type CreateInput struct {
	Name            string
	Description     *string
	SelfJoinEnabled bool
	ActorUserID     string
	Now             time.Time
	IP              *string
	UserAgent       *string
}

type ArchiveInput struct {
	ClassID     string
	Archived    bool
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type AddMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type RemoveMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
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

type RotateRequest struct {
	ClassID       string
	ActorUserID   string
	ExpiresInDays *int
	MaxUses       *int
	IP            string
	UserAgent     string
}

// Rotated is the ONE time the plaintext exists outside the caller's browser.
type Rotated struct {
	Code      string // grouped XXXX-XXXX, for display
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
}

type RevokeRequest struct {
	ClassID     string
	ActorUserID string
	IP          string
	UserAgent   string
}

// Defaults from §6.1 and O-06. Expiry is the spec's; the use cap is the
// deliberate change -- §6.1 defaults to unlimited, which means a forwarded code
// works until it expires, and forwarding rather than guessing is the realistic
// threat (R-02).
const (
	DefaultExpiryDays = 30
	DefaultMaxUses    = 40
)
