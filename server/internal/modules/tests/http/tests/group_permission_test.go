package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpapi"
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
	gate := httpx.RequirePermission(map[string]access.Requirement{"POST /teacher/question-groups": access.AnyOf(access.ContentQuestionsWrite, access.ContentTestsWrite)},
		resolved{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodPost, "/teacher/question-groups", nil)
	req.Pattern = "POST /teacher/question-groups"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func emptyGroupBody(t *testing.T) *openapi.CreateQuestionGroupJSONRequestBody {
	t.Helper()
	var body openapi.CreateQuestionGroupJSONRequestBody
	input := `{"bundle":{"group":{"id":"` + uuid.NewString() + `","title":"Nhóm","members":[],"stimuli":[],"recordings":[]},"questions":[]}}`
	if err := json.Unmarshal([]byte(input), &body); err != nil {
		t.Fatal(err)
	}
	return &body
}

func TestAGroupWriteCarriesTheCallersScopeAndGrants(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher holding only the tests key": {UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)},
		"an Admin":                             {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			var seen command.CreateGroup
			app := &application.Application{Commands: application.Commands{CreateGroup: cqrs.HandlerFunc[command.CreateGroup, domain.StoredGroup](func(_ context.Context, in command.CreateGroup) (domain.StoredGroup, error) {
				seen = in
				return domain.StoredGroup{}, domain.ErrForbidden
			})}}
			response, err := testshttp.NewTests(app, authoringMedia{}).CreateQuestionGroup(contextAs(t, principal), openapi.CreateQuestionGroupRequestObject{Body: emptyGroupBody(t)})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			if err := response.VisitCreateQuestionGroupResponse(rec); err != nil {
				t.Fatal(err)
			}
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			if seen.Actor.Scope != want {
				t.Errorf("the command got scope %+v, want %+v", seen.Actor.Scope, want)
			}
			if !seen.Grants.SubsetOf(principal.Permissions) || !principal.Permissions.SubsetOf(seen.Grants) {
				t.Errorf("the command got grants %v, want the principal's %v", seen.Grants.Keys(), principal.Permissions.Keys())
			}
			var problem openapi.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusForbidden || problem.Error.Code != openapi.FORBIDDEN {
				t.Errorf("a forbidden group write answered %d %s, want 403 FORBIDDEN", rec.Code, problem.Error.Code)
			}
		})
	}
}

func TestScopeFromContextReadsScopeAllFromTheResolvedPrincipal(t *testing.T) {
	teacher := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	admin := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite, access.ScopeAll)}
	if got := httpapi.ScopeFromContext(contextAs(t, teacher)); got != (access.Scope{UserID: teacher.UserID}) {
		t.Errorf("a teacher's scope is %+v", got)
	}
	if got := httpapi.ScopeFromContext(contextAs(t, admin)); got != (access.Scope{UserID: admin.UserID, All: true}) {
		t.Errorf("an Admin's scope is %+v", got)
	}
	if got := httpapi.ScopeFromContext(context.Background()); got != (access.Scope{}) {
		t.Errorf("no principal gives %+v, want the zero scope, which matches nothing", got)
	}
}
