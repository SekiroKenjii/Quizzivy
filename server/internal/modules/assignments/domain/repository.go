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
// reader reaches.
type Repository interface {
	Delete(ctx context.Context, req Request, now time.Time) error
	List(ctx context.Context, in ListInput) ([]Assignment, paging.Page, error)
	Facets(ctx context.Context, in ListInput) (Facets, error)
	Get(ctx context.Context, scope access.Scope, id string) (Assignment, error)
	Create(ctx context.Context, req Request, in WriteInput) (Assignment, error)
	Update(ctx context.Context, req Request, in WriteInput) (Assignment, error)
	Reopen(ctx context.Context, req Request, closesAt time.Time, reason string, now time.Time) (Assignment, error)
	ForStudent(ctx context.Context, studentID string, now time.Time) (StudentSections, error)
	StudentDetail(ctx context.Context, id, studentID string) (StudentDetail, error)
}
