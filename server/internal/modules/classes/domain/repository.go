package domain

import (
	"context"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/paging"
	"time"
)

// Repository persists classes, their roster and their join codes. A teacher
// reaches only the classes they teach, unless scope.all lifts the rule, and
// another teacher's class answers exactly as a missing one does.
type Repository interface {
	Delete(ctx context.Context, classID string, by actor.Actor, now time.Time) error
	Get(ctx context.Context, scope access.Scope, classID string) (Class, error)
	List(ctx context.Context, in ListInput) ([]Class, paging.Page, error)
	ListMine(ctx context.Context, userID string) ([]MyClass, error)
	Members(ctx context.Context, scope access.Scope, classID string, in MembersInput) ([]Member, paging.Page, error)
	Facets(ctx context.Context, scope access.Scope, query string) (Facets, error)
	Create(ctx context.Context, in CreateInput) (Class, error)
	Update(ctx context.Context, scope access.Scope, classID string, in UpdateInput) (Class, error)
	Archive(ctx context.Context, in ArchiveInput) (Class, error)
	AddMember(ctx context.Context, in AddMemberInput) (Member, error)
	RemoveMember(ctx context.Context, in RemoveMemberInput) error
	Rotate(ctx context.Context, in RotateInput) (IssuedCode, error)
	Revoke(ctx context.Context, in RevokeInput) error
	ActiveCode(ctx context.Context, scope access.Scope, classID string) (StoredCode, error)
	// ActiveCodes reads the active code of each class among classIDs that the
	// scope reaches and that has one, as ActiveCode reads one. A class the
	// scope does not reach, and one without an active code, is absent.
	ActiveCodes(ctx context.Context, scope access.Scope, classIDs []string) (map[string]StoredCode, error)
	// LegacyCodeClasses lists every class whose active join code is a legacy
	// one that has not expired at now, archived classes included, by teacher
	// and then by name.
	LegacyCodeClasses(ctx context.Context, now time.Time) ([]LegacyCodeClass, error)
	// RotateLegacyCode revokes the class's active code and issues the sealed
	// one, which keeps the old one's expiry, use cap, use count and creator,
	// in a single transaction, and reports whether it did. It writes
	// nothing and reports false when, read again under the lock, the class
	// has no active legacy code that outlives in.Now. A transaction the
	// database aborted as a deadlock or serialization victim answers
	// ErrRotationContended.
	RotateLegacyCode(ctx context.Context, in LegacyRotationInput) (bool, error)
	Enrol(ctx context.Context, in EnrolInput) (EnrolResult, error)
	LookupByCode(ctx context.Context, code JoinCodeLookup) (*CodeRow, error)
	// ListAll returns one page of every teacher's classes, newest first, each
	// with its teacher; Code is left for the caller to open.
	ListAll(ctx context.Context, in AdminListInput) ([]AdminClass, paging.Page, error)
	// AdminFacets counts what ListAll's tabs and callout show for in.
	AdminFacets(ctx context.Context, in AdminListInput) (AdminFacets, error)
	// CreateForTeacher creates a class for another teacher, with its code
	// when in.Code is set, in one transaction. A teacher who cannot teach
	// answers ErrInvalidTeacher, and nothing is written.
	CreateForTeacher(ctx context.Context, in CreateForTeacherInput) (Class, error)
	// ChangeTeacher gives a class to another teacher. A missing class answers
	// ErrNotFound and a teacher who cannot teach ErrInvalidTeacher; the class's
	// current teacher changes nothing and writes nothing.
	ChangeTeacher(ctx context.Context, in ChangeTeacherInput) (TeacherChange, error)
}
