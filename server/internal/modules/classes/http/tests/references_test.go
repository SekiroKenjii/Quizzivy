package http_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func TestARefusedClassDeletionNamesWhatHoldsIt(t *testing.T) {
	ctx := contextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)})
	for want, refusal := range map[openapi.ReferencedBy]error{
		"assignments": &domain.ReferencedError{By: domain.ReferencedByAssignments},
		"members":     &domain.ReferencedError{By: domain.ReferencedByMembers},
		"other":       domain.ErrReferenced,
	} {
		app := &application.Application{Commands: application.Commands{
			Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(context.Context, command.Delete) (cqrs.Nothing, error) {
				return cqrs.Nothing{}, refusal
			}),
		}}
		response, err := classeshttp.NewClasses(app).DeleteClass(ctx, openapi.DeleteClassRequestObject{Id: uuid.New()})
		refused, ok := response.(openapi.DeleteClass409JSONResponse)
		if err != nil || !ok || refused.Error.Code != openapi.RESOURCEREFERENCED || refused.Error.Details == nil {
			t.Fatalf("a refusal by %s answered %#v (%v)", want, response, err)
		}
		if got := (*refused.Error.Details)["referencedBy"]; got != want {
			t.Errorf("a refusal by %s names %v", want, got)
		}
	}
}
