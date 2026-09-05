// Package domain is the class context: the Class aggregate, its Members, the
// student's own view of it, and the join code that lets a student enrol —
// JoinCodeManager mints and checks codes, CodeState says when one can still
// be redeemed.
package domain

import (
	"quizzivy/internal/shared/stats"
	"time"
)

type Class struct {
	ID                  string
	Name                string
	Description         *string
	StudentCount        int
	OpenAssignmentCount int
	SelfJoinEnabled     bool
	ArchivedAt          *time.Time
	CreatedAt           time.Time
	JoinCode            *JoinCodeInfo
}

type Member struct {
	UserID       string
	FullName     string
	Email        string
	JoinedVia    string
	JoinedAt     time.Time
	JoinCodeHint *string
	// The same figures G-07 shows, so the roster reads "Bài đã nộp" and "Điểm TB" (G-06).
	Stats stats.Student
}

// MyClass is a class as its student sees it (S-10): no code, no roster.
type MyClass struct {
	ID          string
	Name        string
	Description *string
	TeacherName *string
	JoinedAt    time.Time
}

// JoinCodeInfo is metadata about the ACTIVE code -- never the code itself.
// The plaintext exists once, in the response that created it (§13.3).
type JoinCodeInfo struct {
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

// Facets are G-08's tab counts for the current search, ignoring the status filter.
type Facets struct {
	All      int
	Joinable int
	Archived int
	Students int
}
