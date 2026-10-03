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

func TestUpdatingAQuestionTheCallerCannotReachAnswers404(t *testing.T) {
	app := &application.Application{Commands: application.Commands{Update: cqrs.HandlerFunc[command.Update, domain.Question](func(context.Context, command.Update) (domain.Question, error) {
		return domain.Question{}, domain.ErrNotFound
	})}}
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)}
	response, err := questionshttp.NewQuestions(app, nil).UpdateQuestion(contextAs(t, principal),
		openapi.UpdateQuestionRequestObject{Id: uuid.New(), Body: questionBody[openapi.UpdateQuestionJSONRequestBody](t)})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := response.VisitUpdateQuestionResponse(rec); err != nil {
		t.Fatal(err)
	}
	var problem openapi.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound || problem.Error.Code != openapi.NOTFOUND {
		t.Errorf("an unreachable question answered %d %s, want the contract's 404 NOT_FOUND", rec.Code, problem.Error.Code)
	}
}
