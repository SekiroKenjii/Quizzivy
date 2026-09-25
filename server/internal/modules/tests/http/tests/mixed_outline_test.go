package http_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"
)

func TestMixedOutlineTransportKeepsGroupOrderAndArchivedConflict(t *testing.T) {
	var body openapi.UpdateTestJSONRequestBody
	if err := json.Unmarshal([]byte(`{"expectedUpdatedAt":"2026-09-24T00:00:00Z","outlineFormat":"group_v1","sections":[{"title":"Part","questionIds":["01935000-0000-7000-8000-000000000001"],"units":[{"kind":"group","id":"01935000-0000-7000-8000-000000000002"},{"kind":"question","id":"01935000-0000-7000-8000-000000000001"}]}]}`), &body); err != nil {
		t.Fatal(err)
	}
	app := &application.Application{Commands: application.Commands{Update: cqrs.HandlerFunc[command.Update, domain.Test](func(_ context.Context, in command.Update) (domain.Test, error) {
		if !in.Input.GroupOutline || !in.Input.SetSections || !in.Input.Sections[0].SetUnits || len(in.Input.Sections[0].Units) != 2 || in.Input.Sections[0].Units[0].Kind != "group" {
			t.Fatalf("transport dropped mixed structure: %+v", in.Input)
		}
		if err := in.Input.Validate(); err != nil {
			t.Fatalf("valid structure damaged: %v", err)
		}
		return domain.Test{}, domain.ErrArchived
	})}}
	transport := testshttp.NewTests(app, nil)
	handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: uuid.NewString(), Role: "admin"}, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := transport.UpdateTest(r.Context(), openapi.UpdateTestRequestObject{Id: uuid.New(), Body: &body})
		if err != nil {
			t.Fatal(err)
		}
		if err := out.VisitUpdateTestResponse(w); err != nil {
			t.Fatal(err)
		}
	}))
	req := httptest.NewRequest(http.MethodPatch, "/admin/tests", nil)
	req.Header.Set("Authorization", "Bearer synthetic")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"TEST_ARCHIVED"`) {
		t.Fatalf("unexpected conflict: %d %s", response.Code, response.Body.String())
	}
}
