package domain

import (
	"context"
	"errors"
	"time"
)

// ErrClassNotAllowed means a new account was asked to join a class it may not:
// the role is not a student target, or the caller lacks scope.all.
var ErrClassNotAllowed = errors.New("users: a class is allowed only for a student-target role and with scope.all")

// ErrRoleChanged means the role a new account was to be given changed after the
// caller's right to give it was checked. Nothing was written; the check can be
// made again.
var ErrRoleChanged = errors.New("users: the role changed while the account was being created")

// Delivery is how a new account first gets a way in.
type Delivery string

const (
	// DeliveryEmailLink creates the account without a password and emails it a
	// link to choose one.
	DeliveryEmailLink Delivery = "email_link"
	// DeliveryTemporaryPassword creates the account with a temporary password
	// the creator hands over.
	DeliveryTemporaryPassword Delivery = "temporary_password"
)

// Valid reports whether d is one of the two deliveries.
func (d Delivery) Valid() bool { return d == DeliveryEmailLink || d == DeliveryTemporaryPassword }

// AdminRole is a role as the Users table draws it.
type AdminRole struct {
	ID         string
	Name       string
	BuiltinKey string
	Icon       string
	Color      string
}

// AdminUser is one account as the Admin console's Users table lists it,
// whatever its role.
type AdminUser struct {
	ID           string
	FullName     string
	Email        string
	AvatarKey    *string
	Role         AdminRole
	HasPassword  bool
	HasGoogle    bool
	ClassCount   int
	FirstClass   *string
	LastActiveAt *time.Time
	Status       AccountStatus
	CreatedAt    time.Time
	AvatarURL    string
}

// NewUser is an account to create. Hash is the temporary password's hash for
// DeliveryTemporaryPassword and empty for DeliveryEmailLink; the caller computes
// it, because Argon2id blocks on a semaphore that must not be held across a
// transaction. RoleName is recorded in the audit row beside RoleID, and
// RoleRevision is the revision of the role the caller's right to give it was
// checked against.
type NewUser struct {
	Email        string
	FullName     string
	RoleID       string
	RoleName     string
	RoleRevision int64
	ClassID      string
	Delivery     Delivery
	Hash         string
	Now          time.Time
}

// LinkIssue issues the new account its invite on the transaction that creates
// it, so the account and the message commit together or not at all.
type LinkIssue func(ctx context.Context, tx LinkTx, userID string) error

// AdminUsers is the account directory of the Admin console.
type AdminUsers interface {
	// Create adds the account, enrols it in ClassID when set, runs issue on the
	// same transaction when it is not nil, and writes the user.created audit
	// row. It holds the role FOR SHARE, so no change to its grants lands before
	// the commit. It fails with ErrRoleChanged when the role is no longer at
	// RoleRevision, ErrEmailTaken, ErrClassNotFound when ClassID names no class
	// that is not archived, and whatever issue fails with, having written
	// nothing.
	Create(ctx context.Context, req WriteRequest, in NewUser, issue LinkIssue) (AdminUser, error)
}
