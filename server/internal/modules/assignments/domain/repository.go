package domain

import (
	"context"
	"time"

	"quizzivy/internal/shared/paging"
)

// Repository persists assignments and answers the student-side views of them.
type Repository interface {
	List(ctx context.Context, in ListInput) ([]Assignment, paging.Page, error)
	Facets(ctx context.Context, in ListInput) (Facets, error)
	Get(ctx context.Context, id string) (Assignment, error)
	Create(ctx context.Context, req Request, in WriteInput) (Assignment, error)
	Update(ctx context.Context, req Request, in WriteInput) (Assignment, error)
	Reopen(ctx context.Context, req Request, closesAt time.Time, reason string, now time.Time) (Assignment, error)
	ForStudent(ctx context.Context, studentID string, now time.Time) (StudentSections, error)
	StudentDetail(ctx context.Context, id, studentID string) (StudentDetail, error)
}
