package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type reachResolver map[string]access.Principal

func (r reachResolver) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func studentsContextAs(t *testing.T, principal access.Principal) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/students": access.AnyOf(access.WorkspaceTeacher)},
		reachResolver{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/students", nil)
	req.Pattern = "GET /teacher/students"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func TestEveryStudentOperationCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.PeopleStudentsRead, access.PeopleStudentsCreate)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			scopes := map[string]access.Scope{}
			write := func(op string, req domain.WriteRequest) { scopes[op] = req.Scope() }
			app := &application.Application{
				Queries: application.Queries{
					ListStudents: cqrs.HandlerFunc[query.ListStudents, query.ListStudentsResult](func(_ context.Context, q query.ListStudents) (query.ListStudentsResult, error) {
						scopes["list"] = q.Query.Scope
						return query.ListStudentsResult{}, nil
					}),
					StudentFacets: cqrs.HandlerFunc[query.StudentFacets, domain.StudentFacets](func(_ context.Context, q query.StudentFacets) (domain.StudentFacets, error) {
						scopes["facets"] = q.Query.Scope
						return domain.StudentFacets{}, nil
					}),
					GetStudent: cqrs.HandlerFunc[query.GetStudent, domain.Student](func(_ context.Context, q query.GetStudent) (domain.Student, error) {
						scopes["get"] = q.Scope
						return domain.Student{}, domain.ErrStudentNotFound
					}),
				},
				Commands: application.Commands{
					CreateStudent: cqrs.HandlerFunc[command.CreateStudent, command.CreateStudentResult](func(_ context.Context, c command.CreateStudent) (command.CreateStudentResult, error) {
						write("create", c.Request)
						return command.CreateStudentResult{}, domain.ErrClassNotFound
					}),
					UpdateStudent: cqrs.HandlerFunc[command.UpdateStudent, domain.Student](func(_ context.Context, c command.UpdateStudent) (domain.Student, error) {
						write("update", c.Request)
						return domain.Student{}, domain.ErrStudentNotFound
					}),
					ResetStudentPassword: cqrs.HandlerFunc[command.ResetStudentPassword, string](func(_ context.Context, c command.ResetStudentPassword) (string, error) {
						write("reset", c.Request)
						return "", domain.ErrStudentNotFound
					}),
					DeleteStudent: cqrs.HandlerFunc[command.DeleteStudent, cqrs.Nothing](func(_ context.Context, c command.DeleteStudent) (cqrs.Nothing, error) {
						write("delete", c.Request)
						return cqrs.Nothing{}, domain.ErrStudentNotFound
					}),
				},
			}
			h := identityhttp.NewIdentity(app, 0, false, nil)
			ctx := studentsContextAs(t, principal)
			id := uuid.New()
			fullName := "Tên mới"
			responses := map[string]any{}
			for op, call := range map[string]func() (any, error){
				"list": func() (any, error) { return h.ListStudents(ctx, openapi.ListStudentsRequestObject{}) },
				"get":  func() (any, error) { return h.GetStudent(ctx, openapi.GetStudentRequestObject{Id: id}) },
				"create": func() (any, error) {
					return h.CreateStudent(ctx, openapi.CreateStudentRequestObject{Body: &openapi.CreateStudentJSONRequestBody{Email: "moi@example.test", FullName: fullName}})
				},
				"update": func() (any, error) {
					return h.UpdateStudent(ctx, openapi.UpdateStudentRequestObject{Id: id, Body: &openapi.UpdateStudentJSONRequestBody{FullName: &fullName}})
				},
				"reset": func() (any, error) {
					return h.ResetStudentPassword(ctx, openapi.ResetStudentPasswordRequestObject{Id: id})
				},
				"delete": func() (any, error) { return h.DeleteUser(ctx, openapi.DeleteUserRequestObject{Id: id}) },
			} {
				response, err := call()
				if err != nil {
					t.Fatalf("%s: %v", op, err)
				}
				responses[op] = response
			}
			for _, op := range []string{"list", "facets", "get", "create", "update", "reset", "delete"} {
				if scopes[op] != want {
					t.Errorf("%s ran in %+v, want %+v", op, scopes[op], want)
				}
			}
			if _, ok := responses["create"].(openapi.CreateStudent404JSONResponse); !ok {
				t.Errorf("a class the caller does not teach answered %T, want the 404 a missing class gets", responses["create"])
			}
		})
	}
}
