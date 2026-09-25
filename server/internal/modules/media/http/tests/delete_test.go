package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteMediaReportsBothPublishedAndGroupReferences(t *testing.T) {
	asset, group, testID := uuid.New(), uuid.New(), uuid.New()
	testRef := testID.String()
	app := &application.Application{Commands: application.Commands{Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(_ context.Context, in command.Delete) (cqrs.Nothing, error) {
		if in.Input.ID != asset.String() || in.Input.ActorID == "" {
			t.Fatal("delete lost request identity")
		}
		return cqrs.Nothing{}, &domain.ReferencedError{
			Tests:  []domain.TestRef{{ID: testRef, Title: "Published", Version: 2}},
			Groups: []domain.GroupRef{{ID: group.String(), Title: "Shared context", TestID: &testRef}},
		}
	})}}
	transport := mediahttp.NewMedia(app)
	handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: uuid.NewString(), Role: "admin"}, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := transport.DeleteMedia(r.Context(), openapi.DeleteMediaRequestObject{Id: asset})
		if err != nil {
			t.Fatal(err)
		}
		if err := response.VisitDeleteMediaResponse(w); err != nil {
			t.Fatal(err)
		}
	}))
	request := httptest.NewRequest(http.MethodDelete, "/admin/media/"+asset.String(), nil)
	request.Header.Set("Authorization", "Bearer fixture")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status: %d", response.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Tests  []openapi.ReferencingTest  `json:"tests"`
				Groups []openapi.ReferencingGroup `json:"groups"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "MEDIA_REFERENCED" || len(body.Error.Details.Tests) != 1 || len(body.Error.Details.Groups) != 1 {
		t.Fatalf("missing protected references: %s", response.Body.String())
	}
	ref := body.Error.Details.Groups[0]
	if ref.Id != group || ref.TestId == nil || *ref.TestId != testID {
		t.Fatalf("wrong group reference: %+v", ref)
	}
}
