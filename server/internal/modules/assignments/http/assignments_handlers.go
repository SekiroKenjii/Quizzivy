package http

import (
	"context"
	"time"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/paging"
)

// Service is the slice of the assignments application this transport needs.
type Service interface {
	List(ctx context.Context, in domain.ListInput) ([]domain.Assignment, paging.Page, error)
	Facets(ctx context.Context, in domain.ListInput) (domain.Facets, error)
	Get(ctx context.Context, id string) (domain.Assignment, error)
	Create(ctx context.Context, req domain.Request, in domain.WriteInput) (domain.Assignment, error)
	Update(ctx context.Context, req domain.Request, in domain.WriteInput) (domain.Assignment, error)
	Reopen(ctx context.Context, req domain.Request, closesAt time.Time, reason string, now time.Time) (domain.Assignment, error)
	ForStudent(ctx context.Context, studentID string, now time.Time) (domain.StudentSections, error)
	StudentDetail(ctx context.Context, id, studentID string) (domain.StudentDetail, error)
}

type Assignments struct {
	assignments Service
}

func NewAssignments(assignments Service) Assignments {
	return Assignments{assignments: assignments}
}
