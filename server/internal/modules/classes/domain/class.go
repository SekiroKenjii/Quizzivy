// Package domain is the class context: the Class aggregate, its Members, the
// student's own view of it, and the join code that lets a student enrol —
// JoinCodeManager mints and checks codes, CodeState says when one can still
// be redeemed.
package domain

import (
	"quizzivy/internal/shared/stats"
	"time"
)

// Class is a class as its teacher reads it. AverageScore is set by the
// handlers that attach it: it is nil when nothing is graded, and the student's
// own views never carry a Class.
type Class struct {
	ID                  string
	Name                string
	Description         *string
	ScheduleLabel       *string
	Room                *string
	StudentCount        int
	OpenAssignmentCount int
	SelfJoinEnabled     bool
	ArchivedAt          *time.Time
	CreatedAt           time.Time
	JoinCode            *JoinCodeInfo
	AverageScore        *stats.ClassScore
}

// ListedClass is a row of the teacher's class list: the class and, when the
// list was asked for codes and the caller may read this one, its code. Code is
// empty when it was not opened or cannot be read.
type ListedClass struct {
	Class
	Code string
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

// MyClass is a class as its student sees it (S-10): when and where it meets
// and who teaches it, with no code, no count and no roster. TeacherAvatarKey
// is the stored key of the teacher's photo, which the handler signs into
// TeacherAvatarURL.
type MyClass struct {
	ID               string
	Name             string
	Description      *string
	TeacherName      *string
	TeacherAvatarKey *string
	TeacherAvatarURL *string
	ScheduleLabel    *string
	Room             *string
	JoinedAt         time.Time
}

// JoinCodeInfo is the metadata of a class's active join code, without the
// code; getJoinCode reads the code itself (§13.3).
type JoinCodeInfo struct {
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
	// Legacy is true for a code issued before v0.8.0, which only a hash holds.
	Legacy bool
}

// Facets are G-08's tab counts for the current search, ignoring the status filter.
type Facets struct {
	All      int
	Joinable int
	Archived int
	Students int
}
