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
// every student under scope.all; a zero Scope matches nothing. A ClassID the
// scope does not reach matches nothing either.
type StudentQuery struct {
	// StudentStatus defaults to StudentsActive when empty.
	Status  StudentStatus
	Query   string
	ClassID string
	Page    int
	Limit   int
	Scope   access.Scope
}

// WriteRequest is the actor behind a write, for the audit row and for reach: a
// write touches only students ActorID reaches and classes ActorID teaches,
// unless All, the actor's scope.all, is set.
type WriteRequest struct {
	ActorID   string
	All       bool
	IP        string
	UserAgent string
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
