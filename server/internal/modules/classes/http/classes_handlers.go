package http

import (
	"context"

	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/paging"
)

// Service is the slice of the classes application this transport needs.
type Service interface {
	Get(ctx context.Context, classID string) (domain.Class, error)
	List(ctx context.Context, in domain.ListInput) ([]domain.Class, paging.Page, error)
	ListMine(ctx context.Context, userID string) ([]domain.MyClass, error)
	Members(ctx context.Context, classID string, in domain.MembersInput) ([]domain.Member, paging.Page, error)
	Update(ctx context.Context, classID string, in domain.UpdateInput) (domain.Class, error)
	Facets(ctx context.Context, query string) (domain.Facets, error)
	Create(ctx context.Context, name string, description *string, selfJoin bool, actorID, ip, userAgent string) (domain.Class, error)
	Archive(ctx context.Context, classID string, archived bool, actorID, ip, userAgent string) (domain.Class, error)
	RemoveMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) error
	AddMember(ctx context.Context, classID, userID, actorID, ip, userAgent string) (domain.Member, error)
}

// Enrolment is the join-code side of the module: issuing, revoking and redeeming codes.
type Enrolment interface {
	Rotate(ctx context.Context, req domain.RotateRequest) (domain.Rotated, error)
	Revoke(ctx context.Context, req domain.RevokeRequest) error
	Preview(ctx context.Context, rawCode string) (domain.PreviewResult, error)
	EnrolExisting(ctx context.Context, userID, rawCode string, meta domain.Meta) (domain.EnrolResult, error)
}

type Classes struct {
	classes   Service
	enrolment Enrolment
}

func NewClasses(classes Service, enrolment Enrolment) Classes {
	return Classes{classes: classes, enrolment: enrolment}
}
