package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type oneUser map[string]access.Principal

func (o oneUser) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return o[userID], nil
}

func contextAs(t *testing.T, principal access.Principal) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /auth/me": access.AnyOf(access.Self)}, oneUser{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Pattern = "GET /auth/me"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func currentUserApp(user domain.User) *application.Application {
	get := func(context.Context, query.CurrentUser) (domain.User, error) { return user, nil }
	return &application.Application{Queries: application.Queries{
		CurrentUser: cqrs.HandlerFunc[query.CurrentUser, domain.User](get),
	}}
}

func TestTheSignedInUserSeesTheirPermissionsAndWorkspaces(t *testing.T) {
	teacher := access.NewSet(
		access.ContentTestsWrite, access.ContentTestsPublish, access.ContentQuestionsWrite, access.ContentMediaWrite, access.ContentShare,
		access.TeachingClassesWrite, access.TeachingAssignmentsWrite, access.TeachingGrading, access.TeachingAttemptsIntervene, access.TeachingAttendance,
		access.PeopleStudentsRead, access.PeopleStudentsCreate, access.PeopleStudentsResetPassword,
	)
	assistant := access.NewSet(
		access.ContentTestsWrite, access.ContentQuestionsWrite, access.TeachingAssignmentsWrite,
		access.TeachingGrading, access.TeachingAttendance, access.PeopleStudentsRead,
	)
	cases := []struct {
		name        string
		permissions access.Set
		workspaces  []openapi.Workspace
	}{
		{"admin", access.NewSet(access.All()...).Without(access.LearningTakeTests), []openapi.Workspace{openapi.WorkspaceTeacher, openapi.WorkspaceAdmin}},
		{"teacher", teacher, []openapi.Workspace{openapi.WorkspaceTeacher}},
		{"assistant", assistant, []openapi.Workspace{openapi.WorkspaceTeacher}},
		{"student", access.NewSet(access.LearningTakeTests), []openapi.Workspace{openapi.WorkspaceApp}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := "01935000-0000-7000-8000-0000000000a1"
			user := domain.User{ID: id, Email: c.name + "@example.com", FullName: "Nguyễn Văn An", CreatedAt: time.Now()}
			h := identityhttp.NewIdentity(currentUserApp(user), time.Hour, true, nil)
			resp, err := h.GetCurrentUser(contextAs(t, access.Principal{UserID: id, Permissions: c.permissions}), openapi.GetCurrentUserRequestObject{})
			if err != nil {
				t.Fatal(err)
			}
			body, ok := resp.(openapi.GetCurrentUser200JSONResponse)
			if !ok {
				t.Fatalf("GetCurrentUser answered %T", resp)
			}
			want := make([]openapi.PermissionKey, 0, c.permissions.Len())
			for _, k := range c.permissions.Keys() {
				want = append(want, openapi.PermissionKey(k))
			}
			if !slices.Equal(body.Permissions, want) {
				t.Errorf("permissions = %v, want %v in catalogue order", body.Permissions, want)
			}
			if !slices.Equal(body.Workspaces, c.workspaces) {
				t.Errorf("workspaces = %v, want %v", body.Workspaces, c.workspaces)
			}
			wire, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(wire, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["role"]; ok {
				t.Errorf("the response carries a role: %s", wire)
			}
		})
	}
}
