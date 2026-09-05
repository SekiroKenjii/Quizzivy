package domain

import (
	"errors"
	"quizzivy/internal/shared/stats"
	"time"
)

var ErrNotFound = errors.New("classes: not found")

// ListInput selects a page of classes. Query matches the name, accent-folded
// on both sides like every other search here (D-11).
type ListInput struct {
	Query string
	Page  int
	Limit int
	// One of active (the default), joinable, archived, all.
	Status string
}

// MembersInput selects a page of one class's roster. Query matches name or
// email.
type MembersInput struct {
	Query string
	Page  int
	Limit int
}

// JoinCodeInfo is metadata about the ACTIVE code -- never the code itself.
// The plaintext exists once, in the response that created it (§13.3).
type JoinCodeInfo struct {
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

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

// Facets are G-08's tab counts for the current search, ignoring the status filter.
type Facets struct {
	All      int
	Joinable int
	Archived int
	Students int
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

type RemoveMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// UpdateInput carries only the fields the caller actually sent, so a PATCH that
// renames a class cannot silently clear its description.
type UpdateInput struct {
	Name *string
	// nil means "the caller did not send it".
	Description     *string
	SelfJoinEnabled *bool
}

type CreateInput struct {
	Name            string
	Description     *string
	SelfJoinEnabled bool
	ActorUserID     string
	Now             time.Time
	IP              *string
	UserAgent       *string
}

type ArchiveInput struct {
	ClassID     string
	Archived    bool
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

type AddMemberInput struct {
	ClassID     string
	UserID      string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

var ErrNotAStudent = errors.New("classes: not a student")
