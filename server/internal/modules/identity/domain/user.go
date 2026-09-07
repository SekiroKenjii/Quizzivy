// Package domain is the identity context: the User aggregate with its
// credentials and linked providers, the refresh-token families that are its
// sessions, the Student view the teacher administers, and PasswordManager.
package domain

import (
	"fmt"
	classesdomain "quizzivy/internal/modules/classes/domain"
	"time"
)

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

// JoinCodeRejected is a join code that did not pass. It carries the outcome so
// the HTTP layer can reuse /join/preview's exact mapping: the same four codes,
// the same leak rules, decided in one place rather than two that drift.
type JoinCodeRejected struct {
	Outcome classesdomain.PreviewOutcome
}

func (e JoinCodeRejected) Error() string {
	return fmt.Sprintf("join code rejected (outcome %d)", e.Outcome)
}

// HasPassword reports whether the account can log in with a password at all.
// False for Google-only accounts (§5.1).
func (u User) HasPassword() bool { return u.PasswordHash != nil && *u.PasswordHash != "" }

// Disabled reports whether the account is suspended.
func (u User) Disabled() bool { return u.DisabledAt != nil }

// Name bounds, matching users_full_name_check and the contract. A name is
// trimmed before it is measured: " " is not a name.
const (
	MinFullNameLength = 1
	MaxFullNameLength = 200
)
