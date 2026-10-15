package domain

import (
	"context"
	"errors"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
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

// AdminTab is a tab of the Users table. The role tabs include disabled accounts.
type AdminTab string

const (
	// TabAll lists every account.
	TabAll AdminTab = "all"
	// TabTeachers lists every account that is neither an Admin nor a student target.
	TabTeachers AdminTab = "teachers"
	// TabStudents lists the student targets: a role holding at most learning.take_tests.
	TabStudents AdminTab = "students"
	// TabAdmins lists the built-in Admin role.
	TabAdmins AdminTab = "admins"
	// TabDisabled lists every disabled account.
	TabDisabled AdminTab = "disabled"
)

// AdminUserQuery is one page of the Users table. Status is empty for any, or
// AccountMustChangePassword or AccountNeverSignedIn; a disabled account is
// never in either, since AccountStatus puts it first.
type AdminUserQuery struct {
	Tab    AdminTab
	Status AccountStatus
	Query  string
	Email  string
	Page   int
	Limit  int
}

// AdminUserFacets are the tabs' counts under the query's other filters.
type AdminUserFacets struct {
	All, Teachers, Students, Admins, Disabled int
}

// AdminUserRef names an account.
type AdminUserRef struct {
	ID       string
	FullName string
}

// AdminClassRef names a class.
type AdminClassRef struct {
	ID   string
	Name string
}

// OwnedContent counts what an account owns and a transfer would move.
type OwnedContent struct {
	Classes, Tests, Questions, Media, Assignments int
}

// UserEventKind is what a line of an account's recent activity records.
type UserEventKind string

const (
	// EventSignIn is the account's latest sign-in; Detail is its method.
	EventSignIn UserEventKind = "sign_in"
	// EventAudit is a write the account made; Detail is the audit action.
	EventAudit UserEventKind = "audit"
	// EventSubmission is a test the account handed in; Detail is its title.
	EventSubmission UserEventKind = "submission"
	// EventClassJoined is a class the account joined; Detail is its name.
	EventClassJoined UserEventKind = "class_joined"
)

// UserEvent is one line of an account's recent activity.
type UserEvent struct {
	Kind   UserEventKind
	At     time.Time
	Detail *string
}

// MaxUserEvents and MaxUserClasses bound the sheet's two lists.
const (
	MaxUserEvents  = 8
	MaxUserClasses = 20
)

// AdminUserDetail is one account with what the Users sheet draws beside it.
// LastActiveAdmin is true when the account is an active Admin and no other
// active Admin exists.
type AdminUserDetail struct {
	User               AdminUser
	CreatedBy          *AdminUserRef
	Classes            []AdminClassRef
	Owned              OwnedContent
	InProgressAttempts int
	Events             []UserEvent
	LastActiveAdmin    bool
}

// UserCan is what a caller may do to an account, as the guards answer.
type UserCan struct {
	ChangeRole, ResetPassword, Disable, SignOut, Transfer bool
}

// Can applies the guards of an operation on the account to the caller: the
// subset rule against target, the permissions the operation needs beyond
// people.users.manage, not acting on oneself where that is refused, and the
// last active Admin, which matters only to a caller who holds every permission
// without being an Admin. target is the account's effective permissions.
func (d AdminUserDetail) Can(by WriteRequest, target access.Set) UserCan {
	self := d.User.ID == by.ActorID
	subset := access.CanActOn(by.Grants, target)
	disabled := d.User.Status == AccountDisabled
	return UserCan{
		ChangeRole:    subset && !self && !d.LastActiveAdmin && by.Grants.Has(access.PeopleRolesManage),
		ResetPassword: subset && !self,
		Disable:       subset && !self && !d.LastActiveAdmin && !disabled,
		SignOut:       subset,
		Transfer:      by.All && disabled,
	}
}

// AdminUsers is the account directory of the Admin console.
type AdminUsers interface {
	// List returns one page of the accounts the query matches, newest first.
	List(ctx context.Context, in AdminUserQuery) ([]AdminUser, paging.Page, error)
	// Facets counts the tabs under the query's filters, Tab apart.
	Facets(ctx context.Context, in AdminUserQuery) (AdminUserFacets, error)
	// Detail reads one account, whatever its role, or fails with ErrUserNotFound.
	Detail(ctx context.Context, id string) (AdminUserDetail, error)

	// Create adds the account, enrols it in ClassID when set, runs issue on the
	// same transaction when it is not nil, and writes the user.created audit
	// row. It holds the role FOR SHARE, so no change to its grants lands before
	// the commit. It fails with ErrRoleChanged when the role is no longer at
	// RoleRevision, ErrEmailTaken, ErrClassNotFound when ClassID names no class
	// that is not archived, and whatever issue fails with, having written
	// nothing.
	Create(ctx context.Context, req WriteRequest, in NewUser, issue LinkIssue) (AdminUser, error)
}
