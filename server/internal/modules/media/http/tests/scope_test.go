package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type resolved map[string]access.Principal

func (r resolved) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func contextAs(t *testing.T, principal access.Principal) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/media": access.AnyOf(access.WorkspaceTeacher)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/media", nil)
	req.Pattern = "GET /teacher/media"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func principals() map[string]access.Principal {
	return map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentMediaWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	}
}

func TestTheLibraryListsAndTotalsInTheCallersScope(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			var listed, totalled access.Scope
			app := &application.Application{Queries: application.Queries{
				List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
					listed = q.Input.Scope
					return query.ListResult{}, nil
				}),
				TotalBytes: cqrs.HandlerFunc[query.TotalBytes, int64](func(_ context.Context, q query.TotalBytes) (int64, error) {
					totalled = q.Scope
					return 0, nil
				}),
			}}
			if _, err := mediahttp.NewMedia(app).ListMedia(contextAs(t, principal), openapi.ListMediaRequestObject{}); err != nil {
				t.Fatal(err)
			}
			if listed != want || totalled != want {
				t.Errorf("the library listed in %+v and totalled in %+v, want %+v for both", listed, totalled, want)
			}
		})
	}
}

func TestADeleteCarriesTheCallersScopeAll(t *testing.T) {
	for name, principal := range principals() {
		t.Run(name, func(t *testing.T) {
			var seen domain.DeleteInput
			app := &application.Application{Commands: application.Commands{Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(_ context.Context, in command.Delete) (cqrs.Nothing, error) {
				seen = in.Input
				return cqrs.Nothing{}, domain.ErrNotFound
			})}}
			response, err := mediahttp.NewMedia(app).DeleteMedia(contextAs(t, principal), openapi.DeleteMediaRequestObject{Id: uuid.New()})
			if err != nil {
				t.Fatal(err)
			}
			if seen.ActorID != principal.UserID || seen.All != principal.Permissions.Has(access.ScopeAll) {
				t.Errorf("the delete ran as %s with All=%v, want %s with All=%v", seen.ActorID, seen.All, principal.UserID, principal.Permissions.Has(access.ScopeAll))
			}
			if _, ok := response.(openapi.DeleteMedia404JSONResponse); !ok {
				t.Errorf("an asset outside the caller's scope answered %T, want the 404 a missing asset gets", response)
			}
		})
	}
}
