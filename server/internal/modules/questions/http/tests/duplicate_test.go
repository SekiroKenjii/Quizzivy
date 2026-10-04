package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func duplicateFailingWith(t *testing.T, failure error) (int, openapi.ErrorResponse) {
	t.Helper()
	app := &application.Application{Commands: application.Commands{Duplicate: cqrs.HandlerFunc[command.Duplicate, domain.Question](func(context.Context, command.Duplicate) (domain.Question, error) {
		return domain.Question{}, failure
	})}}
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	response, err := questionshttp.NewQuestions(app, nil).DuplicateQuestion(contextAs(t, principal),
		openapi.DuplicateQuestionRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatalf("the duplicate answered no contract response: %v", err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitDuplicateQuestionResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	return rec.Code, problem
}

func TestDuplicatingAQuestionWhoseAssetIsGoneAnswers400NamingTheAsset(t *testing.T) {
	status, problem := duplicateFailingWith(t, domain.ErrMediaNotFound)
	if status != http.StatusBadRequest || problem.Error.Code != openapi.VALIDATIONFAILED {
		t.Fatalf("a source whose asset is gone answered %d %s, want 400 VALIDATION_FAILED as a create answers", status, problem.Error.Code)
	}
	if problem.Error.Details == nil {
		t.Fatal("the answer carries no details, want details.mediaAssetId")
	}
	if _, named := (*problem.Error.Details)["mediaAssetId"]; !named {
		t.Errorf("details = %v, want mediaAssetId named", *problem.Error.Details)
	}
}

func TestDuplicatingAnInvalidSourceAnswers400WithItsFields(t *testing.T) {
	status, problem := duplicateFailingWith(t, &domain.ValidationError{Fields: []domain.FieldError{{Field: "options", Message: "x"}}})
	if status != http.StatusBadRequest || problem.Error.Code != openapi.VALIDATIONFAILED {
		t.Fatalf("a source that no longer validates answered %d %s, want 400 VALIDATION_FAILED as a create answers", status, problem.Error.Code)
	}
	if problem.Error.Details == nil {
		t.Fatal("the answer carries no details, want details.options")
	}
	if got := (*problem.Error.Details)["options"]; got != "x" {
		t.Errorf("details.options = %v, want the field's message", got)
	}
}

func TestDuplicatingAnUnreachableQuestionStillAnswers404(t *testing.T) {
	status, problem := duplicateFailingWith(t, domain.ErrNotFound)
	if status != http.StatusNotFound || problem.Error.Code != openapi.NOTFOUND {
		t.Errorf("an unreachable question answered %d %s, want the contract's 404 NOT_FOUND", status, problem.Error.Code)
	}
}
