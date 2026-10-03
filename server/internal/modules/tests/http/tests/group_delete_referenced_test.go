package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func TestADeleteRefusedByAReferenceAnswers409ResourceReferenced(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	app := &application.Application{Commands: application.Commands{DeleteGroup: cqrs.HandlerFunc[command.DeleteGroup, cqrs.Nothing](func(context.Context, command.DeleteGroup) (cqrs.Nothing, error) {
		return cqrs.Nothing{}, domain.ErrReferenced
	})}}
	response, err := testshttp.NewTests(app, authoringMedia{}).DeleteQuestionGroup(contextAs(t, principal), openapi.DeleteQuestionGroupRequestObject{Id: uuid.New(), Params: openapi.DeleteQuestionGroupParams{ExpectedRevision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitDeleteQuestionGroupResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || problem.Error.Code != openapi.RESOURCEREFERENCED {
		t.Errorf("a delete refused by a reference answered %d %s, want 409 RESOURCE_REFERENCED", rec.Code, problem.Error.Code)
	}
}
