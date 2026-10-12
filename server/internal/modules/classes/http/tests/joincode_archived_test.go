package http_test

import (
	"context"
	"fmt"
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

func rotatingFails(err error) *application.Application {
	return &application.Application{Commands: application.Commands{
		Rotate: cqrs.HandlerFunc[command.Rotate, domain.Rotated](func(context.Context, command.Rotate) (domain.Rotated, error) {
			return domain.Rotated{}, err
		}),
	}}
}

func TestRotatingTheCodeOfAnArchivedClassAnswers409InTheCallersLanguage(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)}
	for _, c := range []struct {
		accept string
		want   string
	}{
		{"", "Lớp này đã được lưu trữ nên không thể tạo mã tham gia mới. Hãy khôi phục lớp trước."},
		{"en", "This class is archived, so it cannot have a new join code. Restore the class first."},
	} {
		for name, err := range map[string]error{
			"as it comes": domain.ErrClassArchived,
			"wrapped":     fmt.Errorf("rotate: %w", domain.ErrClassArchived),
		} {
			h := classeshttp.NewClasses(rotatingFails(err))
			response, callErr := h.RotateJoinCode(contextAsIn(t, principal, c.accept), openapi.RotateJoinCodeRequestObject{Id: uuid.New()})
			if callErr != nil {
				t.Fatalf("%s with Accept-Language %q: %v", name, c.accept, callErr)
			}
			refused, ok := response.(openapi.RotateJoinCode409JSONResponse)
			if !ok {
				t.Fatalf("%s with Accept-Language %q answered %T, want 409", name, c.accept, response)
			}
			if refused.Error.Code != openapi.CLASSARCHIVED {
				t.Errorf("%s with Accept-Language %q answered %s, want CLASS_ARCHIVED", name, c.accept, refused.Error.Code)
			}
			if refused.Error.Message != c.want {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", name, c.accept, refused.Error.Message, c.want)
			}
		}
	}
}

func TestRotatingTheCodeOfAClassTheCallerDoesNotReachAnswers404NotArchived(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)}
	h := classeshttp.NewClasses(rotatingFails(domain.ErrClassNotFound))
	response, err := h.RotateJoinCode(contextAsIn(t, principal, ""), openapi.RotateJoinCodeRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := response.(openapi.RotateJoinCode404JSONResponse); !ok {
		t.Errorf("a class the caller does not reach answered %T, want 404", response)
	}
}
