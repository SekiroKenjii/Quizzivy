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

func TestDeletingATestRefusedByAReferenceAnswers409ResourceReferenced(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	app := &application.Application{Commands: application.Commands{Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(context.Context, command.Delete) (cqrs.Nothing, error) {
		return cqrs.Nothing{}, domain.ErrReferenced
	})}}
	response, err := testshttp.NewTests(app, authoringMedia{}).DeleteTest(contextAs(t, principal), openapi.DeleteTestRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitDeleteTestResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || problem.Error.Code != openapi.RESOURCEREFERENCED {
		t.Errorf("a test delete refused by a reference answered %d %s, want 409 RESOURCE_REFERENCED", rec.Code, problem.Error.Code)
	}
}

func TestAGroupUpdateRefusedAsAReferenceAnswers422WithItsRule(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	app := &application.Application{Commands: application.Commands{UpdateGroup: cqrs.HandlerFunc[command.UpdateGroup, domain.StoredGroup](func(context.Context, command.UpdateGroup) (domain.StoredGroup, error) {
		return domain.StoredGroup{}, &domain.GroupError{Rule: "group_reference"}
	})}}
	id := uuid.New()
	var body openapi.UpdateQuestionGroupJSONRequestBody
	input := `{"bundle":{"group":{"id":"` + id.String() + `","title":"Nhóm","members":[],"stimuli":[],"recordings":[]},"questions":[]},"expectedRevision":1}`
	if err := json.Unmarshal([]byte(input), &body); err != nil {
		t.Fatal(err)
	}
	response, err := testshttp.NewTests(app, authoringMedia{}).UpdateQuestionGroup(contextAs(t, principal), openapi.UpdateQuestionGroupRequestObject{Id: id, Body: &body})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitUpdateQuestionGroupResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnprocessableEntity || problem.Error.Code != openapi.VALIDATIONFAILED {
		t.Errorf("a group update refused as a reference answered %d %s, want 422 VALIDATION_FAILED", rec.Code, problem.Error.Code)
	}
	if problem.Error.Details == nil || (*problem.Error.Details)["rule"] != "group_reference" {
		t.Errorf("the refusal carries details %v, want rule group_reference", problem.Error.Details)
	}
}
