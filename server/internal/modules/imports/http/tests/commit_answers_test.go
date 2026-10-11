package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/validation"
)

func TestACommitOfABadDraftAnswers422ValidationFailed(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	app := &application.Application{Commands: application.Commands{Commit: cqrs.HandlerFunc[command.Commit, command.CommitResult](func(context.Context, command.Commit) (command.CommitResult, error) {
		return command.CommitResult{}, domain.ErrBadDraft
	})}}
	response, err := importshttp.New(app).CommitWordImport(signedIn(t, principal), openapi.CommitWordImportRequestObject{Id: uuid.New(), Body: &openapi.CommitWordImportJSONRequestBody{RequestId: uuid.New(), DraftRevision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitCommitWordImportResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnprocessableEntity || problem.Error.Code != openapi.VALIDATIONFAILED {
		t.Errorf("a commit of a bad draft answered %d %s, want 422 VALIDATION_FAILED", rec.Code, problem.Error.Code)
	}
}

func TestACommitWhoseQuestionComposingLeavesOverALimitAnswers400NamingTheField(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	refused := &validation.Error{Fields: []validation.Field{{Field: "mediaAlt", Message: "Văn bản thay thế không được dài quá 1000 ký tự."}}}
	app := &application.Application{Commands: application.Commands{Commit: cqrs.HandlerFunc[command.Commit, command.CommitResult](func(context.Context, command.Commit) (command.CommitResult, error) {
		return command.CommitResult{}, fmt.Errorf("materialise: %w", refused)
	})}}
	response, err := importshttp.New(app).CommitWordImport(signedIn(t, principal), openapi.CommitWordImportRequestObject{Id: uuid.New(), Body: &openapi.CommitWordImportJSONRequestBody{RequestId: uuid.New(), DraftRevision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitCommitWordImportResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	details := map[string]interface{}{}
	if problem.Error.Details != nil {
		details = *problem.Error.Details
	}
	if rec.Code != http.StatusBadRequest || problem.Error.Code != openapi.VALIDATIONFAILED || details["mediaAlt"] == nil {
		t.Errorf("a refused question answered %d %s %v, want 400 VALIDATION_FAILED naming mediaAlt", rec.Code, problem.Error.Code, details)
	}
}
