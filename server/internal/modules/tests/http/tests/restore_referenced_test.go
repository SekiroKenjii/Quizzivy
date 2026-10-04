package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
	"testing"

	"github.com/google/uuid"
)

func TestARestoreRefusedByAReferenceAnswers409WithItsOwnSentence(t *testing.T) {
	const (
		restoreVI     = "Không thể thay bản nháp vì một câu hỏi trong nhóm của bản nháp vẫn đang được dùng ở nơi khác."
		restoreEN     = "The draft cannot be replaced because a question in one of its groups is still used elsewhere."
		versionDelete = "Phiên bản đã được bài giao hoặc bài làm sử dụng nên không thể xoá."
	)
	app := &application.Application{Commands: application.Commands{
		CreateDraftFromVersion: cqrs.HandlerFunc[command.CreateDraftFromVersion, domain.Test](func(context.Context, command.CreateDraftFromVersion) (domain.Test, error) {
			return domain.Test{}, domain.ErrDraftReferenced
		}),
		DeleteVersion: cqrs.HandlerFunc[command.DeleteVersion, cqrs.Nothing](func(context.Context, command.DeleteVersion) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, domain.ErrReferenced
		}),
	}}
	transport := testshttp.NewTests(app, nil)
	restore := func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.CreateDraftFromTestVersion(ctx, openapi.CreateDraftFromTestVersionRequestObject{Id: uuid.New(), Version: 1, Body: &openapi.CreateDraftFromTestVersionJSONRequestBody{}})
		if err != nil {
			return err
		}
		return out.VisitCreateDraftFromTestVersionResponse(w)
	}
	deleteVersion := func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.DeleteTestVersion(ctx, openapi.DeleteTestVersionRequestObject{Id: uuid.New(), Version: 1})
		if err != nil {
			return err
		}
		return out.VisitDeleteTestVersionResponse(w)
	}
	cases := []struct {
		name, accept, want, other string
		call                      func(context.Context, http.ResponseWriter) error
	}{
		{"a restore", "", restoreVI, versionDelete, restore},
		{"a restore asked for in English", "en", restoreEN, versionDelete, restore},
		{"a version delete", "", versionDelete, restoreVI, deleteVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authenticated := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
				return httpx.Principal{UserID: uuid.NewString()}, nil
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := tc.call(r.Context(), w); err != nil {
					t.Fatal(err)
				}
			}))
			handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(authenticated)
			req := httptest.NewRequest(http.MethodPost, "/teacher/tests", nil)
			req.Header.Set("Authorization", "Bearer synthetic")
			if tc.accept != "" {
				req.Header.Set("Accept-Language", tc.accept)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			var problem openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("%s answered %d %s: %v", tc.name, response.Code, response.Body.String(), err)
			}
			if response.Code != http.StatusConflict || problem.Error.Code != openapi.RESOURCEREFERENCED {
				t.Fatalf("%s refused by a reference answered %d %s, want 409 RESOURCE_REFERENCED", tc.name, response.Code, problem.Error.Code)
			}
			if problem.Error.Message != tc.want {
				t.Errorf("%s refused by a reference says %q, want %q", tc.name, problem.Error.Message, tc.want)
			}
			if problem.Error.Message == tc.other {
				t.Errorf("%s refused by a reference borrowed the other refusal's sentence: %q", tc.name, problem.Error.Message)
			}
		})
	}
}
