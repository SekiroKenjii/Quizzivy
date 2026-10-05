package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/paging"
)

type resolved map[string]access.Principal

func (r resolved) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func principals() map[string]access.Principal {
	return map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingGrading)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	}
}

func gated(t *testing.T, principal access.Principal, pattern string) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{pattern: access.AnyOf(access.WorkspaceTeacher)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/attempts", nil)
	req.Pattern = pattern
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

type recordingHome struct{ seen *access.Scope }

func (r recordingHome) Summary(_ context.Context, scope access.Scope) (domain.Summary, error) {
	*r.seen = scope
	return domain.Summary{}, nil
}

func (recordingHome) List(context.Context, domain.ListQuery) ([]domain.Recent, paging.Page, error) {
	return nil, paging.Page{}, nil
}

func TestTheHomeCarriesTheCallersScopeToTheStore(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			ctx := gated(t, principal, "GET /teacher/dashboard")
			var seen access.Scope
			app := application.New(recordingHome{seen: &seen})
			if _, err := dashboardhttp.NewDashboard(app).GetDashboard(ctx, openapi.GetDashboardRequestObject{}); err != nil {
				t.Fatal(err)
			}
			if want := (access.Scope{UserID: principal.UserID}); seen != want {
				t.Errorf("the home ran in %+v, want %+v", seen, want)
			}
		})
	}
}

func TestTheAttemptListCarriesTheCallersScope(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			ctx := gated(t, principal, "GET /teacher/attempts")
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

func (r recordingHome) Home(_ context.Context, q domain.HomeQuery) (domain.Home, error) {
	*r.seen = q.Scope
	return domain.Home{}, nil
}
func (recordingHome) LiveAssignments(context.Context, access.Scope, time.Time) (int, error) {
	return 0, nil
}
func (recordingHome) AnswersToGrade(context.Context, access.Scope) (int, error) { return 0, nil }
