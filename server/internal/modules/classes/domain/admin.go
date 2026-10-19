package domain

import (
	"errors"
	"time"
)

// ErrInvalidTeacher is a teacher named for a class who does not exist, is
// disabled or holds no key that opens the teacher workspace.
var ErrInvalidTeacher = errors.New("classes: the teacher must be an active account with the teacher workspace")

// AdminListInput selects a page of every teacher's classes for the Admin
// console. Tab is active (the default) or archived; Query matches the class's
// name or its teacher's, accent-folded on both sides (D-11); DisabledTeacher
// keeps the classes whose teacher's account is disabled.
type AdminListInput struct {
	Query           string
	Tab             string
	DisabledTeacher bool
	Page            int
	Limit           int
}

// ClassTeacher is the teacher of a class as the Admin console names them.
// AvatarKey is the stored key of their photo, which the handler signs into
// AvatarURL.
type ClassTeacher struct {
	ID        string
	Name      string
	AvatarKey *string
	AvatarURL *string
	Disabled  bool
}

// AdminClass is a row of the Admin console's class list. Code is the active
// code read in full, empty when there is none or it cannot be read.
type AdminClass struct {
	Class
	Teacher ClassTeacher
	Code    string
}

// AdminFacets are the Admin list's tab counts for its search and teacher
// filter, and DisabledTeacherClasses the classes that are not archived and
// whose teacher is disabled, whatever the search, tab and filter.
type AdminFacets struct {
	Active                 int
	Archived               int
	DisabledTeacherClasses int
}

// NewCode is a sealed join code issued with a class in the transaction that
// creates it: CodeID and the class's id are chosen before the insert because
// the ciphertext is bound to both.
type NewCode struct {
	CodeID     string
	CodeHash   []byte
	Ciphertext []byte
	KeyID      int16
	Hint       string
	ExpiresAt  time.Time
	MaxUses    *int
}

// CreateForTeacherInput creates the class ClassID that TeacherID teaches, on
// behalf of ActorUserID, with Code when one is issued.
type CreateForTeacherInput struct {
	ClassID       string
	Name          string
	ScheduleLabel *string
	Room          *string
	TeacherID     string
	Code          *NewCode
	ActorUserID   string
	Now           time.Time
	IP            *string
	UserAgent     *string
}

// ChangeTeacherInput gives the class ClassID to TeacherID.
type ChangeTeacherInput struct {
	ClassID     string
	TeacherID   string
	ActorUserID string
	Now         time.Time
	IP          *string
	UserAgent   *string
}

// TeacherChange is what a change of teacher did. Changed is false when the
// class already had that teacher, and Previous is nil for a class that named
// none; PreviousDisabled says the previous teacher's account is disabled, and
// TeacherName names the new teacher.
type TeacherChange struct {
	Class            Class
	Changed          bool
	Previous         *string
	PreviousDisabled bool
	TeacherName      string
}
