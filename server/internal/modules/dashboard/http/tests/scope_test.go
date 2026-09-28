package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type resolved map[string]access.Principal

func (r resolved) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func TestTheAttemptListCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingGrading)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			var ctx context.Context
			verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
			gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/attempts": access.AnyOf(access.WorkspaceTeacher)},
				resolved{principal.UserID: principal})
			h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
			req := httptest.NewRequest(http.MethodGet, "/teacher/attempts", nil)
			req.Pattern = "GET /teacher/attempts"
			req.Header.Set("Authorization", "Bearer token")
			h.ServeHTTP(httptest.NewRecorder(), req)
			if ctx == nil {
				t.Fatal("the gate did not pass the request")
			}
			var seen access.Scope
			app := &application.Application{Queries: application.Queries{List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
				seen = q.Query.Scope
				return query.ListResult{}, nil
			})}}
			if _, err := dashboardhttp.NewDashboard(app).ListAttempts(ctx, openapi.ListAttemptsRequestObject{}); err != nil {
				t.Fatal(err)
			}
			if want := (access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}); seen != want {
				t.Errorf("the list ran in %+v, want %+v", seen, want)
			}
		})
	}
}
