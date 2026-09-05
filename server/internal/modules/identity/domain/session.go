package domain

import (
	"time"
)

type RefreshTokenRecord struct {
	UserID    string
	FamilyID  string
	TokenHash []byte
	IssuedAt  time.Time
	ExpiresAt time.Time
	UserAgent *string
	IP        *string
}

// RotateOutcome classifies a presented refresh token. Rotate returns one of
// these rather than an error for the non-OK cases: "this token was reused" is a
// normal, expected result that the caller must act on, not a fault.
type RotateOutcome int

const (
	// RotateOK: the token was live and has been consumed; its successor exists.
	RotateOK RotateOutcome = iota
	// RotateUnknown: no such token. Never issued, or already pruned.
	RotateUnknown
	// RotateExpired: issued by us, but aged out. Not an attack.
	RotateExpired
	RotateReused
	RotateRevoked
	// RotateUserDisabled: the account was suspended after the token was issued.
	RotateUserDisabled
)

type RotateResult struct {
	Outcome  RotateOutcome
	User     User
	FamilyID string
}

// ChangePasswordRecord is what the store needs to swap a password and prune the
// sessions that the old one authorised.
type ChangePasswordRecord struct {
	UserID        string
	NewHash       string
	KeepTokenHash []byte
	Now           time.Time
	IP            *string
	UserAgent     *string
}
