package domain

import (
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/stats"
	"time"
)

// Student is §7's User narrowed to the role this listing returns, plus what
// G-07 draws beside it: the memberships and figures the reader's scope reaches.
type Student struct {
	ID                 string
	Email              string
	FullName           string
	HasPassword        bool
	LinkedProviders    []string
	MustChangePassword bool
	CreatedAt          time.Time
	DisabledAt         *time.Time
	Classes            []Membership
	Stats              stats.Student
}

// Account is one user's account fields, whatever their role, with the legacy
// role derived from what that role holds now: "student" for a student-like
// role and "admin" otherwise, as tokens and /auth/me derive it.
type Account struct {
	DisplayName        *string
	ID                 string
	Email              string
	FullName           string
	Role               string
	HasPassword        bool
	LinkedProviders    []string
	MustChangePassword bool
	CreatedAt          time.Time
}

// Membership is one class the student is in, and how they got there (D-10).
type Membership struct {
	ID        string
	Name      string
	JoinedVia string
	JoinedAt  time.Time
}

// StudentFacets are G-07's header: "31 học viên · 23 hoạt động 7 ngày qua".
type StudentFacets struct {
	Total           int
	ActiveLast7Days int
}

// StudentStatus selects which accounts a listing returns.
type StudentStatus string

const (
	StudentsActive   StudentStatus = "active"
	StudentsDisabled StudentStatus = "disabled"
	StudentsAny      StudentStatus = "all"
)

// StudentQuery selects the students Scope reaches (visibility.StudentIDs), or
// every student under scope.all; a zero Scope matches nothing. ClassIDs, when
// not empty, keeps the students who are in at least one of those classes; a
// class the scope does not reach matches nothing. MustChange, when set, keeps
// the accounts whose must_change_password equals it.
type StudentQuery struct {
	// StudentStatus defaults to StudentsActive when empty.
	Status     StudentStatus
	Query      string
	ClassIDs   []string
	MustChange *bool
	Page       int
	Limit      int
	Scope      access.Scope
}

// WriteRequest is the actor behind a write, for the audit row, for reach and
// for the guards that are not permissions: a write touches only students
// ActorID reaches and classes ActorID teaches, unless All, the actor's
// scope.all, is set. Grants are the actor's effective permissions; they decide
// whether the actor manages accounts and whether a student's permissions are a
// subset of the actor's.
type WriteRequest struct {
	ActorID   string
	All       bool
	Grants    access.Set
	IP        string
	UserAgent string
}

// ManagesUsers reports whether the actor holds people.users.manage, which
// disables and enables accounts and lifts the shared-student guard.
func (r WriteRequest) ManagesUsers() bool {
	return r.Grants.Has(access.PeopleUsersManage)
}

// Scope is the scope the request reads under: ActorID's own, or everyone's
// with All.
func (r WriteRequest) Scope() access.Scope {
	return access.Scope{UserID: r.ActorID, All: r.All}
}

type NewStudent struct {
	Email    string
	FullName string
	ClassIDs []string
	// Hash is computed by the caller: Argon2id blocks on a four-slot semaphore
	// that must not be held across a transaction.
	Hash string
	Now  time.Time
}

type StudentPatch struct {
	ID string
	// nil means the caller did not send the field.
	FullName *string
	Email    *string
	Disabled *bool
	Now      time.Time
}

// MaxBulkReset is how many students one bulk password reset may name.
const MaxBulkReset = 40

// PasswordReset is one student a bulk reset reset: the temporary password is
// shown once and kept nowhere.
type PasswordReset struct {
	StudentID         string
	FullName          string
	Email             string
	TemporaryPassword string
}

// ResetFailure names a student a bulk reset did not reset, and why: Reason is
// ErrStudentNotFound, ErrForbidden, ErrStudentShared or ErrResetFailed.
type ResetFailure struct {
	StudentID string
	Reason    error
}

// BulkReset is what a bulk password reset did, each list in the order the
// students were named.
type BulkReset struct {
	Reset  []PasswordReset
	Failed []ResetFailure
}
