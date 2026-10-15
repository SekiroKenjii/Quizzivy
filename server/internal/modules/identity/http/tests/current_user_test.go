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
			var role map[string]json.RawMessage
			if err := json.Unmarshal(fields["role"], &role); err != nil {
				t.Errorf("role is not the role record, but %s", fields["role"])
			}
		})
	}
}

func TestTheSignedInUserSeesTheirOwnRoleAndNoOtherField(t *testing.T) {
	id := "01935000-0000-7000-8000-0000000000a1"
	roleID := "01935000-0000-7000-8000-0000000000b2"
	cases := []struct {
		name    string
		role    domain.Role
		builtin string
		stored  string
	}{
		{"a built-in role", domain.Role{ID: roleID, BuiltinKey: new("teacher"), Name: "Giáo viên"}, `"teacher"`, "Giáo viên"},
		{"a renamed built-in role", domain.Role{ID: roleID, BuiltinKey: new("admin"), Name: "Giám đốc"}, `"admin"`, "Giám đốc"},
		{"a custom role", domain.Role{ID: roleID, Name: "Trợ lý phòng đề"}, `null`, "Trợ lý phòng đề"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			user := domain.User{ID: id, Email: "an@example.com", FullName: "Nguyễn Văn An", CreatedAt: time.Now(), Role: c.role}
			h := identityhttp.NewIdentity(currentUserApp(user), time.Hour, true, nil)
			resp, err := h.GetCurrentUser(contextAs(t, access.Principal{UserID: id, Permissions: access.NewSet(access.ContentTestsWrite)}), openapi.GetCurrentUserRequestObject{})
			if err != nil {
				t.Fatal(err)
			}
			body, ok := resp.(openapi.GetCurrentUser200JSONResponse)
			if !ok {
				t.Fatalf("GetCurrentUser answered %T", resp)
			}
			wire, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Role map[string]json.RawMessage `json:"role"`
			}
			if err := json.Unmarshal(wire, &doc); err != nil {
				t.Fatal(err)
			}
			if got := string(doc.Role["builtinKey"]); got != c.builtin {
				t.Errorf("builtinKey = %s, want %s", got, c.builtin)
			}
			if got := string(doc.Role["id"]); got != `"`+roleID+`"` {
				t.Errorf("id = %s, want %q", got, roleID)
			}
			var name string
			if err := json.Unmarshal(doc.Role["name"], &name); err != nil || name != c.stored {
				t.Errorf("name = %s, want %q", doc.Role["name"], c.stored)
			}
			if len(doc.Role) != 3 {
				t.Errorf("the role record carries %d fields, want id, builtinKey and name: %s", len(doc.Role), wire)
			}
		})
	}
}
