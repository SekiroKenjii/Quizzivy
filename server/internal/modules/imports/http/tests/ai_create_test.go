package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

func creating(t *testing.T, h func(*application.Application) importshttp.Imports, by access.Principal, read *bool) (*command.Create, *httptest.ResponseRecorder) {
	t.Helper()
	var seen *command.Create
	now := time.Now().UTC()
	app := &application.Application{Commands: application.Commands{Create: cqrs.HandlerFunc[command.Create, domain.Import](func(_ context.Context, in command.Create) (domain.Import, error) {
		seen = &in
		return domain.Import{ID: uuid.NewString(), CreatedBy: by.UserID, Title: in.Title, Status: "awaiting_sources", Revision: 1, CreatedAt: now, UpdatedAt: now, ReadWithAi: in.ReadWithAi}, nil
	})}}
	response, err := h(app).CreateWordImport(signedIn(t, by), openapi.CreateWordImportRequestObject{Body: &openapi.CreateWordImportJSONRequestBody{RequestId: uuid.New(), Title: "Đề", ReadWithAi: read}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitCreateWordImportResponse(recorder); err != nil {
		t.Fatal(err)
	}
	return seen, recorder
}

func TestAnImportThatAsksToBeReadWithAIIsRefusedWhereTheAssistantIsOffOrNotAllowed(t *testing.T) {
	yes := true
	teacher := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite, access.AIUse)}
	withoutAI := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	for name, c := range map[string]struct {
		on bool
		by access.Principal
	}{
		"the server has no assistant": {false, teacher},
		"the caller lacks ai.use":     {true, withoutAI},
	} {
		t.Run(name, func(t *testing.T) {
			seen, recorder := creating(t, func(app *application.Application) importshttp.Imports { return importshttp.New(app).WithAI(c.on) }, c.by, &yes)
			var body struct {
				Error struct {
					Code    string
					Details map[string]string
				}
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != 422 || body.Error.Code != "VALIDATION_FAILED" || body.Error.Details["readWithAi"] == "" || seen != nil {
				t.Fatalf("status %d, body %s, created %v", recorder.Code, recorder.Body.String(), seen != nil)
			}
		})
	}
}

func TestAnImportThatAsksToBeReadWithAICarriesTheSwitchAndAnswersWithIt(t *testing.T) {
	yes, no := true, false
	teacher := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite, access.AIUse)}
	enabled := func(app *application.Application) importshttp.Imports { return importshttp.New(app).WithAI(true) }
	for name, c := range map[string]struct {
		asked *bool
		want  bool
	}{"asked": {&yes, true}, "declined": {&no, false}, "absent": {nil, false}} {
		t.Run(name, func(t *testing.T) {
			seen, recorder := creating(t, enabled, teacher, c.asked)
			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			_, present := body["readWithAi"]
			if recorder.Code != 201 || seen == nil || seen.ReadWithAi != c.want || present != c.want {
				t.Fatalf("status %d, body %s, command %+v", recorder.Code, recorder.Body.String(), seen)
			}
		})
	}
}

func TestAnImportThatDoesNotAskNeedsNoAssistantAndNoPermission(t *testing.T) {
	plain := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	seen, recorder := creating(t, func(app *application.Application) importshttp.Imports { return importshttp.New(app) }, plain, nil)
	if recorder.Code != 201 || seen == nil || seen.ReadWithAi {
		t.Fatalf("status %d, command %+v", recorder.Code, seen)
	}
}
