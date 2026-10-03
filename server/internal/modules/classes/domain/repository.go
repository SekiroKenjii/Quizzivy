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
	Enrol(ctx context.Context, in EnrolInput) (EnrolResult, error)
	LookupByCode(ctx context.Context, code JoinCodeLookup) (*CodeRow, error)
}
