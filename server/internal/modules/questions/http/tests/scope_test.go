package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	questionshttp "quizzivy/internal/modules/questions/http"
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
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/questions": access.AnyOf(access.WorkspaceTeacher)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/questions", nil)
	req.Pattern = "GET /teacher/questions"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

type seen struct {
	scopes   map[string]access.Scope
	requests map[string]domain.WriteRequest
}

func recording() (*seen, *application.Application) {
	s := &seen{scopes: map[string]access.Scope{}, requests: map[string]domain.WriteRequest{}}
	write := func(name string) cqrs.HandlerFunc[domain.WriteRequest, domain.Question] {
		return func(_ context.Context, req domain.WriteRequest) (domain.Question, error) {
			s.requests[name] = req
			return domain.Question{}, domain.ErrMediaNotFound
		}
	}
	app := &application.Application{
		Queries: application.Queries{
			List: cqrs.HandlerFunc[query.List, query.ListResult](func(_ context.Context, q query.List) (query.ListResult, error) {
				s.scopes["list"] = q.Input.Scope
				return query.ListResult{}, nil
			}),
			Facets: cqrs.HandlerFunc[query.Facets, domain.TypeFacets](func(_ context.Context, q query.Facets) (domain.TypeFacets, error) {
				s.scopes["facets"] = q.Input.Scope
				return domain.TypeFacets{}, nil
			}),
			Tags: cqrs.HandlerFunc[query.Tags, []string](func(_ context.Context, q query.Tags) ([]string, error) {
				s.scopes["tags"] = q.Input.Scope
				return nil, nil
			}),
			Counts: cqrs.HandlerFunc[query.Counts, query.CountsResult](func(_ context.Context, q query.Counts) (query.CountsResult, error) {
				s.scopes["counts"] = q.Input.Scope
				return query.CountsResult{}, nil
			}),
			Get: cqrs.HandlerFunc[query.Get, domain.Question](func(_ context.Context, q query.Get) (domain.Question, error) {
				s.scopes["get"] = q.Scope
				return domain.Question{}, domain.ErrNotFound
			}),
		},
		Commands: application.Commands{
			Create: cqrs.HandlerFunc[command.Create, domain.Question](func(ctx context.Context, cmd command.Create) (domain.Question, error) {
				return write("create")(ctx, cmd.Request)
			}),
			Update: cqrs.HandlerFunc[command.Update, domain.Question](func(ctx context.Context, cmd command.Update) (domain.Question, error) {
				return write("update")(ctx, cmd.Request)
			}),
			Duplicate: cqrs.HandlerFunc[command.Duplicate, domain.Question](func(_ context.Context, cmd command.Duplicate) (domain.Question, error) {
				s.requests["duplicate"] = cmd.Request
				return domain.Question{}, domain.ErrNotFound
			}),
			Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(_ context.Context, cmd command.Delete) (cqrs.Nothing, error) {
				s.requests["delete"] = cmd.Request
				return cqrs.Nothing{}, domain.ErrNotFound
			}),
			AddTags: cqrs.HandlerFunc[command.AddTags, int](func(_ context.Context, cmd command.AddTags) (int, error) {
				s.scopes["tag"] = cmd.Scope
				return 0, nil
			}),
		},
	}
	return s, app
}

func questionBody[T any](t *testing.T) *T {
	t.Helper()
	var body T
	if err := json.Unmarshal([]byte(`{"type":"short_answer","prompt":"Câu hỏi","points":1,"tags":[]}`), &body); err != nil {
		t.Fatal(err)
	}
	return &body
}

func TestEveryQuestionOperationCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentQuestionsWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			s, app := recording()
			h := questionshttp.NewQuestions(app, nil)
			ctx := contextAs(t, principal)
			id := uuid.New()
			calls := []func() error{
				func() error { _, err := h.ListQuestions(ctx, openapi.ListQuestionsRequestObject{}); return err },
				func() error { _, err := h.GetQuestion(ctx, openapi.GetQuestionRequestObject{Id: id}); return err },
				func() error {
					_, err := h.TagQuestions(ctx, openapi.TagQuestionsRequestObject{Body: &openapi.TagQuestionsJSONRequestBody{QuestionIds: []openapi.Uuid{id}, Tags: []string{"the"}}})
					return err
				},
				func() error {
					_, err := h.CreateQuestion(ctx, openapi.CreateQuestionRequestObject{Body: questionBody[openapi.CreateQuestionJSONRequestBody](t)})
					return err
				},
				func() error {
					_, err := h.UpdateQuestion(ctx, openapi.UpdateQuestionRequestObject{Id: id, Body: questionBody[openapi.UpdateQuestionJSONRequestBody](t)})
					return err
				},
				func() error {
					_, err := h.DuplicateQuestion(ctx, openapi.DuplicateQuestionRequestObject{Id: id})
					return err
				},
				func() error { _, err := h.DeleteQuestion(ctx, openapi.DeleteQuestionRequestObject{Id: id}); return err },
			}
			for i, call := range calls {
				if err := call(); err != nil {
					t.Fatalf("call %d: %v", i, err)
				}
			}
			for _, read := range []string{"list", "facets", "tags", "counts", "get", "tag"} {
				if s.scopes[read] != want {
					t.Errorf("%s ran in %+v, want %+v", read, s.scopes[read], want)
				}
			}
			for _, write := range []string{"create", "update", "duplicate", "delete"} {
				if got := s.requests[write]; got.ActorID != want.UserID || got.All != want.All {
					t.Errorf("%s ran as %s with All=%v, want %s with All=%v", write, got.ActorID, got.All, want.UserID, want.All)
				}
			}
		})
	}
}
