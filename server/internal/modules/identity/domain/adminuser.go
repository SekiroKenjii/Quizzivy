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

// ErrRoleChanged means a role the write depends on changed after the caller's
// right to use it was checked: the role a new account was to be given, or the
// role an account held when its permissions were read. Nothing was written;
// the check can be made again.
var ErrRoleChanged = errors.New("users: a role changed while the account was being written")

// ErrCannotActOnSelf means the operation is refused on the caller's own
// account: a role change, a disable or a password reset. The caller changes
// their own password through ChangePassword, and may sign themselves out.
var ErrCannotActOnSelf = errors.New("users: this operation is refused on the caller's own account")

// OwnsContentError means an account that owns classes or content was given a
// role that opens no teacher workspace. Owned counts what it owns, and nothing
// was written.
type OwnsContentError struct {
	Owned OwnedContent
}

// Error implements error.
func (*OwnsContentError) Error() string {
	return "users: the account owns classes or content and the new role opens no teacher workspace"
}

// ErrActionFailed means a bulk action met a fault with one account that is not
// a refusal; the cause is in the server log.
var ErrActionFailed = errors.New("users: the action failed for this account")

// UserFailure is an account a bulk action did not reach, and the refusal or
// fault that kept it from the account.
type UserFailure struct {
	UserID string
	Reason error
}

// UserReset is an account a bulk reset reset. TemporaryPassword is set for
// DeliveryTemporaryPassword only.
type UserReset struct {
	UserID            string
	FullName          string
	Email             string
	TemporaryPassword string
}

// BulkUserReset is what a bulk reset did: the accounts it reset and those it
// did not, each in the order the request named them.
type BulkUserReset struct {
	Reset  []UserReset
	Failed []UserFailure
}

// BulkDisable is what a bulk disable did: the ids of the accounts that are
// disabled now and those it did not reach, each in the order the request named
// them.
type BulkDisable struct {
	Disabled []string
	Failed   []UserFailure
}

// MaxBulkDisable and MaxBulkLinkReset bound a bulk disable and a bulk reset by
// email link; MaxBulkPasswordReset, the same as MaxBulkReset of the students'
// bulk reset, bounds one by temporary password, because each is hashed.
const (
	MaxBulkDisable       = 100
	MaxBulkLinkReset     = 100
	MaxBulkPasswordReset = MaxBulkReset
)

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

// Any reports whether the account owns anything a transfer would move.
func (o OwnedContent) Any() bool {
	return o.Classes+o.Tests+o.Questions+o.Media+o.Assignments > 0
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
	ChangeRole, ResetPassword, Disable, Enable, SignOut, Transfer bool
}

// Can applies the guards of an operation on the account to the caller: the
// subset rule against target, the permissions the operation needs beyond
// people.users.manage, not acting on oneself where that is refused, a disabled
// account where an operation refuses one, and the last active Admin. The
// last-Admin term is defence in depth: no caller can reach it today, because an
// Admin caller either is the account or leaves another Admin, and a custom role
// is never a superset of the Admin's permissions (scope.all and three more
// cannot be granted to one). target is the account's effective permissions.
func (d AdminUserDetail) Can(by WriteRequest, target access.Set) UserCan {
	self := d.User.ID == by.ActorID
	subset := access.CanActOn(by.Grants, target)
	disabled := d.User.Status == AccountDisabled
	return UserCan{
		ChangeRole:    subset && !self && !d.LastActiveAdmin && by.Grants.Has(access.PeopleRolesManage),
		ResetPassword: subset && !self && !disabled,
		Disable:       subset && !self && !d.LastActiveAdmin && !disabled,
		Enable:        subset && disabled,
		SignOut:       subset,
		Transfer:      by.All && disabled,
	}
}

// AccountWrite names the account a write acts on and the role it held when the
// caller's right to act on it was checked. The write holds the account locked,
// and fails with ErrRoleChanged when the role is another by then, so a check
// made against a role the account has since left never authorises a write.
type AccountWrite struct {
	UserID string
	RoleID string
	Now    time.Time
}

// RoleChange moves an account to ToRoleID, a role the caller's right to give
// was checked at ToRoleRevision. ToAdmin and ToOpensTeacher say what that role
// is: the built-in Admin, and a role whose permissions open the teacher
// workspace.
type RoleChange struct {
	AccountWrite
	ToRoleID       string
	ToRoleName     string
	ToRoleRevision int64
	ToAdmin        bool
	ToOpensTeacher bool
}

// AccountPasswordReset replaces an account's way in. Hash is the temporary
// password's hash for DeliveryTemporaryPassword, computed by the caller
// because Argon2id blocks on a semaphore that must not be held across a
// transaction, and empty for DeliveryEmailLink, which clears the password and
// runs Issue on the same transaction.
type AccountPasswordReset struct {
	AccountWrite
	Delivery Delivery
	Hash     string
	Issue    LinkIssue
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

	// Get reads one account as the table lists it, or fails with ErrUserNotFound.
	Get(ctx context.Context, id string) (AdminUser, error)
	// ChangeRole gives the account the role, ends its sessions and writes the
	// user.role_changed audit row. It holds the new role FOR SHARE, every
	// active Admin and the account FOR NO KEY UPDATE, in that order, and fails
	// with ErrUserNotFound, ErrRoleChanged when either role is no longer the
	// one checked, ErrLastAdmin for the only active Admin leaving the Admin
	// role, and *OwnsContentError. The role the account has already changes
	// nothing.
	ChangeRole(ctx context.Context, req WriteRequest, in RoleChange) error
	// Disable sets disabled_at, ends the account's sessions, revokes its unused
	// set-password links, cancels the messages waiting for it and writes the
	// user.disabled audit row, holding every active Admin and the account
	// locked. It fails with ErrUserNotFound, ErrRoleChanged and ErrLastAdmin;
	// an account disabled already changes nothing.
	Disable(ctx context.Context, req WriteRequest, in AccountWrite) error
	// Enable clears disabled_at and writes the user.enabled audit row. It fails
	// with ErrUserNotFound and ErrRoleChanged; an account enabled already
	// changes nothing.
	Enable(ctx context.Context, req WriteRequest, in AccountWrite) error
	// SignOut ends the account's sessions and writes the
	// user.signed_out_everywhere audit row. It fails with ErrUserNotFound and
	// ErrRoleChanged.
	SignOut(ctx context.Context, req WriteRequest, in AccountWrite) error
	// ResetPassword replaces the account's password as AccountPasswordReset says, ends
	// its sessions, revokes its unused set-password links and writes the
	// user.password_reset audit row. It fails with ErrUserNotFound,
	// ErrRoleChanged, ErrAccountDisabled, ErrSetPasswordUnavailable for a link
	// on a server that cannot send one, and whatever Issue fails with, having
	// written nothing.
	ResetPassword(ctx context.Context, req WriteRequest, in AccountPasswordReset) error
}
