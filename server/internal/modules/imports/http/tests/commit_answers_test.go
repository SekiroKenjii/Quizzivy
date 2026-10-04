package http_test

import (
	"context"
	"encoding/json"
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
