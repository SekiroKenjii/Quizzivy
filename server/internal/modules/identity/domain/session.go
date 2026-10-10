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
	GeoLabel  *string
}

// SessionBasis is the account state a sign-in was decided on: the session
// epoch its access token carries and the password hash it read, nil for an
// account without a password.
type SessionBasis struct {
	Epoch        int
	PasswordHash *string
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

// RenameRecord is an account changing its own display name. Nothing else about
// the account travels with it: the email is the login and only an admin moves it.
type RenameRecord struct {
	UserID    string
	FullName  string
	Now       time.Time
	IP        *string
	UserAgent *string
}

// MaxSessions bounds the list of a user's live sessions. The current one is
// always among them; of the others the most recently used come first.
const MaxSessions = 100

// Session is one live sign-in of a user: a refresh-token family that is
// neither revoked nor expired. UserAgent and GeoLabel are those of the latest
// sign-in or refresh in the family, and LastUsedAt is its time. Current is true
// for the family the request's refresh cookie names.
type Session struct {
	FamilyID   string
	UserAgent  *string
	GeoLabel   *string
	LastUsedAt time.Time
	Current    bool
}

// SessionsQuery asks for a user's live sessions at Now. CurrentTokenHash is the
// hash of the refresh cookie the request carried, nil when it carried none.
type SessionsQuery struct {
	UserID           string
	CurrentTokenHash []byte
	Now              time.Time
	Limit            int
}

// RevokeSessionRecord is a user ending one of their own live sessions.
// CurrentTokenHash is the hash of the refresh cookie the request carried.
type RevokeSessionRecord struct {
	UserID           string
	FamilyID         string
	CurrentTokenHash []byte
	Now              time.Time
	IP               *string
	UserAgent        *string
}

// RevokeOtherSessionsRecord is a user ending every live session of theirs but
// the one the request's refresh cookie names.
type RevokeOtherSessionsRecord struct {
	UserID           string
	CurrentTokenHash []byte
	Now              time.Time
	IP               *string
	UserAgent        *string
}
