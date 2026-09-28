package http_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func refusing(err error) *application.Application {
	return &application.Application{Commands: application.Commands{
		UpdateStudent: cqrs.HandlerFunc[command.UpdateStudent, domain.Student](func(context.Context, command.UpdateStudent) (domain.Student, error) {
			return domain.Student{}, err
		}),
		ResetStudentPassword: cqrs.HandlerFunc[command.ResetStudentPassword, string](func(context.Context, command.ResetStudentPassword) (string, error) {
			return "", err
		}),
		DeleteStudent: cqrs.HandlerFunc[command.DeleteStudent, cqrs.Nothing](func(context.Context, command.DeleteStudent) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, err
		}),
	}}
}

func TestTheGuardsAnswer403WithTheirCode(t *testing.T) {
	ctx := studentsContextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.PeopleStudentsCreate, access.PeopleStudentsResetPassword)})
	id := uuid.New()
	name := "Tên mới"
	for _, c := range []struct {
		err  error
		code openapi.ErrorCode
	}{{domain.ErrForbidden, openapi.FORBIDDEN}, {domain.ErrStudentShared, openapi.STUDENTSHARED}} {
		h := identityhttp.NewIdentity(refusing(c.err), 0, false, nil)
		updated, err := h.UpdateStudent(ctx, openapi.UpdateStudentRequestObject{Id: id, Body: &openapi.UpdateStudentJSONRequestBody{FullName: &name}})
		if refused, ok := updated.(openapi.UpdateStudent403JSONResponse); err != nil || !ok || refused.Error.Code != c.code {
			t.Errorf("updateStudent on %v answered %#v (%v), want 403 %s", c.err, updated, err, c.code)
		}
		reset, err := h.ResetStudentPassword(ctx, openapi.ResetStudentPasswordRequestObject{Id: id})
		if refused, ok := reset.(openapi.ResetStudentPassword403JSONResponse); err != nil || !ok || refused.Error.Code != c.code {
			t.Errorf("resetStudentPassword on %v answered %#v (%v), want 403 %s", c.err, reset, err, c.code)
		}
	}
	deleted, err := identityhttp.NewIdentity(refusing(domain.ErrForbidden), 0, false, nil).DeleteUser(ctx, openapi.DeleteUserRequestObject{Id: id})
	if refused, ok := deleted.(openapi.DeleteUser403JSONResponse); err != nil || !ok || refused.Error.Code != openapi.FORBIDDEN {
		t.Errorf("deleteUser on the subset rule answered %#v (%v), want 403 FORBIDDEN", deleted, err)
	}
}

func TestARefusedUserDeletionNamesWhatHoldsIt(t *testing.T) {
	ctx := studentsContextAs(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)})
	for want, err := range map[openapi.ReferencedBy]error{
		"owned_content": &domain.ReferencedError{By: domain.ReferencedByOwnedContent},
		"audit":         &domain.ReferencedError{By: domain.ReferencedByAudit},
		"other":         domain.ErrReferenced,
	} {
		response, callErr := identityhttp.NewIdentity(refusing(err), 0, false, nil).DeleteUser(ctx, openapi.DeleteUserRequestObject{Id: uuid.New()})
		refused, ok := response.(openapi.DeleteUser409JSONResponse)
		if callErr != nil || !ok || refused.Error.Code != openapi.RESOURCEREFERENCED || refused.Error.Details == nil {
			t.Fatalf("a refusal by %s answered %#v (%v)", want, response, callErr)
		}
		if got := (*refused.Error.Details)["referencedBy"]; got != want {
			t.Errorf("a refusal by %s names %v", want, got)
		}
	}
}
