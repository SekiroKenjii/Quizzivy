package http_test

import (
	"context"
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

	"github.com/google/uuid"
)

func TestVersionAndLegacyOutlineConflictsAreActionableHTTPResponses(t *testing.T) {
	app := &application.Application{Commands: application.Commands{
		CreateDraftFromVersion: cqrs.HandlerFunc[command.CreateDraftFromVersion, domain.Test](func(context.Context, command.CreateDraftFromVersion) (domain.Test, error) {
			return domain.Test{}, domain.ErrArchived
		}),
		SetCurrentVersion: cqrs.HandlerFunc[command.SetCurrentVersion, domain.Test](func(context.Context, command.SetCurrentVersion) (domain.Test, error) {
			return domain.Test{}, domain.ErrArchived
		}),
		Update: cqrs.HandlerFunc[command.Update, domain.Test](func(context.Context, command.Update) (domain.Test, error) {
			return domain.Test{}, domain.ErrGroupOutlineRequired
		}),
	}}
	transport := testshttp.NewTests(app, nil)
	cases := []struct {
		name, code string
		call       func(context.Context, http.ResponseWriter) error
	}{
		{"restore", "TEST_ARCHIVED", func(ctx context.Context, w http.ResponseWriter) error {
			out, err := transport.CreateDraftFromTestVersion(ctx, openapi.CreateDraftFromTestVersionRequestObject{Id: uuid.New(), Version: 1, Body: &openapi.CreateDraftFromTestVersionJSONRequestBody{}})
			if err != nil {
				return err
			}
			return out.VisitCreateDraftFromTestVersionResponse(w)
		}},
		{"default", "TEST_ARCHIVED", func(ctx context.Context, w http.ResponseWriter) error {
			out, err := transport.SetCurrentTestVersion(ctx, openapi.SetCurrentTestVersionRequestObject{Id: uuid.New(), Version: 1, Body: &openapi.SetCurrentTestVersionJSONRequestBody{}})
			if err != nil {
				return err
			}
			return out.VisitSetCurrentTestVersionResponse(w)
		}},
		{"outline", "GROUP_OUTLINE_REQUIRED", func(ctx context.Context, w http.ResponseWriter) error {
			out, err := transport.UpdateTest(ctx, openapi.UpdateTestRequestObject{Id: uuid.New(), Body: &openapi.UpdateTestJSONRequestBody{}})
			if err != nil {
				return err
			}
			return out.VisitUpdateTestResponse(w)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
				return httpx.Principal{UserID: uuid.NewString(), Role: "admin"}, nil
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := tc.call(r.Context(), w); err != nil {
					t.Fatal(err)
				}
			}))
			req := httptest.NewRequest(http.MethodPost, "/admin/tests", nil)
			req.Header.Set("Authorization", "Bearer synthetic")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("unexpected conflict: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
