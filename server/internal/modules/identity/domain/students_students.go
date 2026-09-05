package domain

import (
	"errors"
	"quizzivy/internal/shared/stats"
	"time"
)

// Membership is one class the student is in, and how they got there (D-10).
type Membership struct {
	ID        string
	Name      string
	JoinedVia string
	JoinedAt  time.Time
}

// Student is §7's User narrowed to the role this listing returns, plus what
// G-07 draws beside it.
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

type StudentQuery struct {
	// StudentStatus defaults to StudentsActive when empty.
	Status  StudentStatus
	Query   string
	ClassID string
	Page    int
	Limit   int
}

var (
	ErrStudentNotFound = errors.New("students: not found")
	ErrEmailTaken      = errors.New("students: email already in use")
)
