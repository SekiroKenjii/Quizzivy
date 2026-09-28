package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	classeshttp "quizzivy/internal/modules/classes/http"
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
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/classes": access.AnyOf(access.WorkspaceTeacher)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/classes", nil)
	req.Pattern = "GET /teacher/classes"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

type seen struct {
	scopes map[string]access.Scope
	owners map[string]string
}

func (s *seen) actor(name, id string, scope access.Scope) {
	s.owners[name] = id
	s.scopes[name] = scope
}

func recording() (*seen, *application.Application) {
	s := &seen{scopes: map[string]access.Scope{}, owners: map[string]string{}}
	app := &application.Application{
		Queries: application.Queries{
			List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
				s.scopes["list"] = q.Input.Scope
				return query.ListResult{}, nil
			}),
			Facets: cqrs.HandlerFunc[query.Facets, domain.Facets](func(_ context.Context, q query.Facets) (domain.Facets, error) {
				s.scopes["facets"] = q.Scope
				return domain.Facets{}, nil
			}),
			Get: cqrs.HandlerFunc[query.Get, domain.Class](func(_ context.Context, q query.Get) (domain.Class, error) {
				s.scopes["get"] = q.Scope
				return domain.Class{}, domain.ErrNotFound
			}),
			Members: cqrs.HandlerFunc[query.Members, query.MembersResult](func(_ context.Context, q query.Members) (query.MembersResult, error) {
				s.scopes["members"] = q.Scope
				return query.MembersResult{}, nil
			}),
			ActiveCode: cqrs.HandlerFunc[query.ActiveCode, domain.ActiveJoinCode](func(_ context.Context, q query.ActiveCode) (domain.ActiveJoinCode, error) {
				s.scopes["joinCode"] = q.Scope
				return domain.ActiveJoinCode{}, domain.ErrClassNotFound
			}),
		},
		Commands: application.Commands{
			Create: cqrs.HandlerFunc[command.Create, domain.Class](func(_ context.Context, c command.Create) (domain.Class, error) {
				s.actor("create", c.Actor.ID, c.Actor.Scope)
				return domain.Class{}, nil
			}),
			Update: cqrs.HandlerFunc[command.Update, domain.Class](func(_ context.Context, c command.Update) (domain.Class, error) {
				s.scopes["update"] = c.Scope
				return domain.Class{}, nil
			}),
			Archive: cqrs.HandlerFunc[command.Archive, domain.Class](func(_ context.Context, c command.Archive) (domain.Class, error) {
				s.actor("archive", c.Actor.ID, c.Actor.Scope)
				return domain.Class{}, nil
			}),
			Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(_ context.Context, c command.Delete) (cqrs.Nothing, error) {
				s.actor("delete", c.Actor.ID, c.Actor.Scope)
				return cqrs.Nothing{}, nil
			}),
			AddMember: cqrs.HandlerFunc[command.AddMember, domain.Member](func(_ context.Context, c command.AddMember) (domain.Member, error) {
				s.actor("add", c.Actor.ID, c.Actor.Scope)
				return domain.Member{}, domain.ErrNotAStudent
			}),
			RemoveMember: cqrs.HandlerFunc[command.RemoveMember, cqrs.Nothing](func(_ context.Context, c command.RemoveMember) (cqrs.Nothing, error) {
				s.actor("remove", c.Actor.ID, c.Actor.Scope)
				return cqrs.Nothing{}, nil
			}),
			Rotate: cqrs.HandlerFunc[command.Rotate, domain.Rotated](func(_ context.Context, c command.Rotate) (domain.Rotated, error) {
				s.actor("rotate", c.Request.ActorUserID, access.Scope{UserID: c.Request.ActorUserID, All: c.Request.All})
				return domain.Rotated{}, nil
			}),
			Revoke: cqrs.HandlerFunc[command.Revoke, cqrs.Nothing](func(_ context.Context, c command.Revoke) (cqrs.Nothing, error) {
				s.actor("revoke", c.Request.ActorUserID, access.Scope{UserID: c.Request.ActorUserID, All: c.Request.All})
				return cqrs.Nothing{}, nil
			}),
		},
	}
	return s, app
}

func TestEveryClassOperationCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingClassesWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			s, app := recording()
			h := classeshttp.NewClasses(app)
			ctx := contextAs(t, principal)
			id := uuid.New()
			archived := true
			newName := "Lớp mới"
			calls := map[string]func() (any, error){
				"list": func() (any, error) { return h.ListClasses(ctx, openapi.ListClassesRequestObject{}) },
				"get":  func() (any, error) { return h.GetClass(ctx, openapi.GetClassRequestObject{Id: id}) },
				"create": func() (any, error) {
					return h.CreateClass(ctx, openapi.CreateClassRequestObject{Body: &openapi.CreateClassJSONRequestBody{Name: newName}})
				},
				"update": func() (any, error) {
					return h.UpdateClass(ctx, openapi.UpdateClassRequestObject{Id: id, Body: &openapi.UpdateClassJSONRequestBody{Name: &newName, Archived: &archived}})
				},
				"members": func() (any, error) { return h.ListClassMembers(ctx, openapi.ListClassMembersRequestObject{Id: id}) },
				"add": func() (any, error) {
					return h.AddClassMember(ctx, openapi.AddClassMemberRequestObject{Id: id, Body: &openapi.AddClassMemberJSONRequestBody{UserId: uuid.New()}})
				},
				"remove": func() (any, error) {
					return h.RemoveClassMember(ctx, openapi.RemoveClassMemberRequestObject{Id: id, UserId: uuid.New()})
				},
				"delete": func() (any, error) { return h.DeleteClass(ctx, openapi.DeleteClassRequestObject{Id: id}) },
				"rotate": func() (any, error) { return h.RotateJoinCode(ctx, openapi.RotateJoinCodeRequestObject{Id: id}) },
				"revoke": func() (any, error) { return h.RevokeJoinCode(ctx, openapi.RevokeJoinCodeRequestObject{Id: id}) },
				"joinCode": func() (any, error) {
					return h.GetJoinCode(ctx, openapi.GetJoinCodeRequestObject{Id: id})
				},
			}
			responses := map[string]any{}
			for op, call := range calls {
				response, err := call()
				if err != nil {
					t.Fatalf("%s: %v", op, err)
				}
				responses[op] = response
			}
			for _, op := range []string{"list", "facets", "get", "update", "members", "create", "archive", "delete", "add", "remove", "rotate", "revoke", "joinCode"} {
				if s.scopes[op] != want {
					t.Errorf("%s ran in %+v, want %+v", op, s.scopes[op], want)
				}
			}
			for op, owner := range s.owners {
				if owner != principal.UserID {
					t.Errorf("%s ran as %s, want %s", op, owner, principal.UserID)
				}
			}
			if _, ok := responses["joinCode"].(openapi.GetJoinCode404JSONResponse); !ok {
				t.Errorf("another teacher's class answered %T, want the 404 a missing class gets", responses["joinCode"])
			}
			if _, ok := responses["add"].(openapi.AddClassMember404JSONResponse); !ok {
				t.Errorf("an unreachable student answered %T, want the 404 a missing account gets", responses["add"])
			}
		})
	}
}
