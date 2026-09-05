package domain

import (
	"time"
)

// Defaults from §6.1 and O-06. Expiry is the spec's; the use cap is the
// deliberate change -- §6.1 defaults to unlimited, which means a forwarded code
// works until it expires, and forwarding rather than guessing is the realistic
// threat (R-02).
const (
	DefaultExpiryDays = 30
	DefaultMaxUses    = 40
)

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

// Meta is the request context §6.5 requires on every enrolment audit row.
type Meta struct {
	IP        string
	UserAgent string
}
