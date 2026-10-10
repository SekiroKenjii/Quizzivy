package domain

import (
	"context"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
	"time"
)

// Repository persists assignments and answers the student-side views of them.
// A teacher reaches the assignments visibility.AssignmentIDs gives them, or
// every one under scope.all; another teacher's answers exactly as a missing
// one does, and the targets it embeds name only the classes and students the
// reader reaches. An override is read and written only for a student the
// teacher reaches. The student-side views are the student's own window, under
// their override when they have one.
type Repository interface {
	Delete(ctx context.Context, req Request, now time.Time) error
	List(ctx context.Context, in ListInput) ([]Assignment, paging.Page, error)
	Facets(ctx context.Context, in ListInput) (Facets, error)
	Get(ctx context.Context, scope access.Scope, id string) (Assignment, error)
	Create(ctx context.Context, req Request, in WriteInput) (Assignment, error)
	Update(ctx context.Context, req Request, in WriteInput) (Assignment, error)
	Reopen(ctx context.Context, req Request, closesAt time.Time, reason string, now time.Time) (Assignment, error)
	Extend(ctx context.Context, req Request, minutes int, notify bool, now time.Time) (Assignment, error)
	SetOverrides(ctx context.Context, req Request, in OverrideInput) ([]StudentOverride, error)
	Overrides(ctx context.Context, scope access.Scope, assignmentID string) ([]StudentOverride, error)
	DeleteOverride(ctx context.Context, req Request, studentID string, now time.Time) error
	ForStudent(ctx context.Context, studentID string, now time.Time) (StudentSections, error)
	StudentDetail(ctx context.Context, id, studentID string) (StudentDetail, error)
	// ClosesMoved reads, for a published assignment whose own close was just
	// moved, every enabled student it is addressed to whose close the move
	// changed: the ones without an override, and those whose override closes
	// before the assignment's new close. A student whose override closes at
	// or after it keeps the close they had.
	ClosesMoved(ctx context.Context, assignmentID string) (Extension, error)
	// ClosesGranted reads, for a published assignment, those of studentIDs it
	// is addressed to, enabled, whose override closes after the assignment's
	// own close: the students an override gave time the assignment does not.
	ClosesGranted(ctx context.Context, assignmentID string, studentIDs []string) (Extension, error)
}
