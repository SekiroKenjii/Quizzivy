package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
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
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/assignments": access.AnyOf(access.WorkspaceTeacher)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/assignments", nil)
	req.Pattern = "GET /teacher/assignments"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func TestEveryAssignmentOperationCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingAssignmentsWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			scopes := map[string]access.Scope{}
			write := func(op string, req domain.Request) { scopes[op] = req.Scope() }
			app := &application.Application{
				Queries: application.Queries{
					List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
						scopes["list"] = q.Input.Scope
						return query.ListResult{}, nil
					}),
					Facets: cqrs.HandlerFunc[query.Facets, domain.Facets](func(_ context.Context, q query.Facets) (domain.Facets, error) {
						scopes["facets"] = q.Input.Scope
						return domain.Facets{}, nil
					}),
					Get: cqrs.HandlerFunc[query.Get, domain.Assignment](func(_ context.Context, q query.Get) (domain.Assignment, error) {
						scopes["get"] = q.Scope
						return domain.Assignment{}, domain.ErrNotFound
					}),
				},
				Commands: application.Commands{
					Create: cqrs.HandlerFunc[command.Create, domain.Assignment](func(_ context.Context, c command.Create) (domain.Assignment, error) {
						write("create", c.Request)
						return domain.Assignment{}, domain.ErrTestNotPublished
					}),
					Update: cqrs.HandlerFunc[command.Update, domain.Assignment](func(_ context.Context, c command.Update) (domain.Assignment, error) {
						write("update", c.Request)
						return domain.Assignment{}, domain.ErrNotFound
					}),
					Reopen: cqrs.HandlerFunc[command.Reopen, domain.Assignment](func(_ context.Context, c command.Reopen) (domain.Assignment, error) {
						write("reopen", c.Request)
						return domain.Assignment{}, domain.ErrNotFound
					}),
					Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(_ context.Context, c command.Delete) (cqrs.Nothing, error) {
						write("delete", c.Request)
						return cqrs.Nothing{}, domain.ErrNotFound
					}),
				},
			}
			h := assignmentshttp.NewAssignments(app)
			ctx := contextAs(t, principal)
			id := uuid.New()
			for op, call := range map[string]func() (any, error){
				"list": func() (any, error) { return h.ListAssignments(ctx, openapi.ListAssignmentsRequestObject{}) },
				"get":  func() (any, error) { return h.GetAssignment(ctx, openapi.GetAssignmentRequestObject{Id: id}) },
				"create": func() (any, error) {
					return h.CreateAssignment(ctx, openapi.CreateAssignmentRequestObject{Body: &openapi.CreateAssignmentJSONRequestBody{}})
				},
				"update": func() (any, error) {
					return h.UpdateAssignment(ctx, openapi.UpdateAssignmentRequestObject{Id: id, Body: &openapi.UpdateAssignmentJSONRequestBody{}})
				},
				"reopen": func() (any, error) {
					return h.ReopenAssignment(ctx, openapi.ReopenAssignmentRequestObject{Id: id, Body: &openapi.ReopenAssignmentJSONRequestBody{ClosesAt: time.Now().Add(time.Hour), Reason: "Gia hạn"}})
				},
				"delete": func() (any, error) { return h.DeleteAssignment(ctx, openapi.DeleteAssignmentRequestObject{Id: id}) },
			} {
				if _, err := call(); err != nil {
					t.Fatalf("%s: %v", op, err)
				}
			}
			for _, op := range []string{"list", "facets", "get", "create", "update", "reopen", "delete"} {
				if scopes[op] != want {
					t.Errorf("%s ran in %+v, want %+v", op, scopes[op], want)
				}
			}
		})
	}
}
