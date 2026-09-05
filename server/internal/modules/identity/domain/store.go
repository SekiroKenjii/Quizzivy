package domain

import (
	"errors"
	"time"
)

var ErrUserNotFound = errors.New("user not found")

// User is the slice of app.users this package needs.
type User struct {
	ID                 string
	Email              string
	FullName           string
	Role               string
	PasswordHash       *string
	MustChangePassword bool
	DisabledAt         *time.Time
	CreatedAt          time.Time
	LinkedProviders    []string
}

// HasPassword reports whether the account can log in with a password at all.
// False for Google-only accounts (§5.1).
func (u User) HasPassword() bool { return u.PasswordHash != nil && *u.PasswordHash != "" }

// Disabled reports whether the account is suspended.
func (u User) Disabled() bool { return u.DisabledAt != nil }

type RefreshTokenRecord struct {
	UserID    string
	FamilyID  string
	TokenHash []byte
	IssuedAt  time.Time
	ExpiresAt time.Time
	UserAgent *string
	IP        *string
}

// ErrIdentityAlreadyLinked means the account already has an identity from this
// provider, and it is a different one. D-08's UNIQUE (user_id, provider) is
// what makes that detectable rather than silently creating a second link.
var ErrIdentityAlreadyLinked = errors.New("identity already linked")
