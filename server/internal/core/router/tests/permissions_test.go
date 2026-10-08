package router_test

import (
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/openapi"
	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/permissions.golden from api/openapi.yaml")

func freshSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func eachOperation(spec *openapi3.T, fn func(pattern string, op *openapi3.Operation)) {
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op != nil {
				fn(method+" "+path, op)
			}
		}
	}
}

func openPatterns(spec *openapi3.T) map[string]struct{} {
	return httpx.OpenRoutes(spec, "bearerAuth")
}

func TestEveryNonOpenOperationDeclaresAPermission(t *testing.T) {
	spec := freshSpec(t)
	open := openPatterns(spec)
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if _, isOpen := open[pattern]; isOpen {
			return
		}
		if _, ok := op.Extensions[httpx.PermissionExtension]; !ok {
			t.Errorf("%s (%s) declares no x-permission", pattern, op.OperationID)
		}
	})
}

func TestOpenOperationsDeclareNone(t *testing.T) {
	spec := freshSpec(t)
	open := openPatterns(spec)
	if len(open) != 7 {
		t.Errorf("%d open operations, want the seven of theOpenSeven", len(open))
	}
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if _, isOpen := open[pattern]; !isOpen {
			return
		}
		if v, ok := op.Extensions[httpx.PermissionExtension]; ok {
			t.Errorf("open operation %s declares x-permission %v", pattern, v)
		}
	})
}

func TestEveryDeclaredValueIsKnown(t *testing.T) {
	requirements, err := httpx.PermissionRequirements(freshSpec(t), "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	if len(requirements) != 109 {
		t.Errorf("%d operations declare a permission, want 109", len(requirements))
	}
	for pattern, requirement := range requirements {
		for _, k := range requirement.Keys() {
			if !k.Known() && !k.Pseudo() {
				t.Errorf("%s requires %q, which is neither a catalogue key nor a pseudo-key", pattern, k)
			}
		}
	}
}

func TestTheAssertionRefusesAnUndeclaredOperation(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		method  string
		value   any
		missing bool
		want    string
	}{
		{name: "undeclared", path: "/teacher/tests", method: "GET", missing: true, want: "GET /teacher/tests: declares no permission"},
		{name: "open and declared", path: "/auth/login", method: "POST", value: "self", want: "POST /auth/login: an open operation declares a permission"},
		{name: "unknown key", path: "/teacher/media", method: "GET", value: "content.everything", want: `GET /teacher/media: "content.everything" is neither a catalogue key nor a pseudo-key`},
		{name: "empty list", path: "/teacher/classes", method: "GET", value: []any{}, want: "GET /teacher/classes: declares an empty list"},
		{name: "repeated key", path: "/teacher/students", method: "GET", value: []any{"people.students.read", "people.students.read"}, want: `GET /teacher/students: repeats "people.students.read"`},
		{name: "hidden key outside admin", path: "/app/classes", method: "GET", value: "scope.all", want: `GET /app/classes: the hidden key "scope.all" is declared outside /admin/`},
		{name: "not a key", path: "/teacher/dashboard", method: "GET", value: 7, want: "GET /teacher/dashboard: declares 7, which is neither a key nor a list of keys"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := freshSpec(t)
			op := spec.Paths.Find(c.path).GetOperation(c.method)
			if op == nil {
				t.Fatalf("no %s %s in the contract", c.method, c.path)
			}
			if c.missing {
				delete(op.Extensions, httpx.PermissionExtension)
			} else {
				if op.Extensions == nil {
					op.Extensions = map[string]any{}
				}
				op.Extensions[httpx.PermissionExtension] = c.value
			}
			_, err := httpx.PermissionRequirements(spec, "bearerAuth")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to name %q", err, c.want)
			}
		})
	}
}

func TestThePermissionMapIsPinned(t *testing.T) {
	requirements, err := httpx.PermissionRequirements(freshSpec(t), "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, len(requirements))
	for pattern, requirement := range requirements {
		keys := make([]string, 0, len(requirement.Keys()))
		for _, k := range requirement.Keys() {
			keys = append(keys, string(k))
		}
		lines = append(lines, pattern+" "+strings.Join(keys, " | "))
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"
	golden := filepath.Join("testdata", "permissions.golden")
	if *updateGolden {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v; run go test -run TestThePermissionMapIsPinned -update", err)
	}
	if got != string(want) {
		t.Errorf("the permission map differs from %s; if the change is intended, run go test -run TestThePermissionMapIsPinned -update and review the diff\n%s",
			golden, lineDiff(string(want), got))
	}
}

func lineDiff(want, got string) string {
	wantSet := map[string]bool{}
	for _, l := range strings.Split(want, "\n") {
		wantSet[l] = true
	}
	gotSet := map[string]bool{}
	for _, l := range strings.Split(got, "\n") {
		gotSet[l] = true
	}
	var out []string
	for l := range wantSet {
		if !gotSet[l] && l != "" {
			out = append(out, "- "+l)
		}
	}
	for l := range gotSet {
		if !wantSet[l] && l != "" {
			out = append(out, "+ "+l)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

const (
	adminUser     = "01935000-0000-7000-8000-0000000000a1"
	teacherUser   = "01935000-0000-7000-8000-0000000000c3"
	assistantUser = "01935000-0000-7000-8000-0000000000c4"
	studentUser   = "01935000-0000-7000-8000-0000000000b2"
)

func rolePrincipals() *fakePrincipals {
	return newFakePrincipals().
		set(builtinPrincipal(adminUser, access.BuiltinAdmin)).
		set(builtinPrincipal(teacherUser, access.BuiltinTeacher)).
		set(builtinPrincipal(assistantUser, access.BuiltinAssistant)).
		set(builtinPrincipal(studentUser, access.BuiltinStudent))
}

func roleRouter(t *testing.T, issuer *identitytoken.Issuer, principals *fakePrincipals) http.Handler {
	t.Helper()
	h, err := router.New(router.Deps{Principals: principals, DB: fakeDB{}, Tokens: issuer},
		slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func sendAs(t *testing.T, h http.Handler, issuer *identitytoken.Issuer, method, path, userID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		token, err := issuer.Issue(userID, "admin", 0)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAStudentCannotReachTheTeacherTree(t *testing.T) {
	issuer := testIssuer(t)
	rec := sendAs(t, roleRouter(t, issuer, rolePrincipals()), issuer, http.MethodGet, "/teacher/dashboard", studentUser, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if code := errorCode(t, rec); code != "FORBIDDEN" {
		t.Errorf("error code = %q, want FORBIDDEN", code)
	}
}

func TestATeacherReachesTheTeacherTree(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	for _, user := range []string{adminUser, teacherUser} {
		if rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/dashboard", user, ""); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s was refused the dashboard: %d", user, rec.Code)
		}
	}
}

func TestEveryStaffOperationIsGated(t *testing.T) {
	spec := freshSpec(t)
	requirements, err := httpx.PermissionRequirements(spec, "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	open := openPatterns(spec)
	var admin int
	eachOperation(spec, func(pattern string, _ *openapi3.Operation) {
		if _, isOpen := open[pattern]; isOpen {
			return
		}
		if _, gated := requirements[pattern]; !gated {
			t.Errorf("%s has no requirement", pattern)
		}
		if strings.Contains(pattern, " /teacher/") || strings.Contains(pattern, " /admin/") {
			admin++
		}
	})
	if admin < 25 {
		t.Fatalf("only %d admin operations found; the derivation is looking at the wrong thing", admin)
	}
}

func TestTheStudentTreeNeedsTakeTests(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	if rec := sendAs(t, h, issuer, http.MethodGet, "/app/assignments", studentUser, ""); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Errorf("a student was refused the student tree: %d", rec.Code)
	}
	if rec := sendAs(t, h, issuer, http.MethodGet, "/app/assignments", adminUser, ""); rec.Code != http.StatusForbidden {
		t.Errorf("an Admin without Take tests got %d on the student tree, want 403", rec.Code)
	}
}

func TestAnAnonymousCallerToTheTeacherTreeGetsAuthenticationNotAuthorization(t *testing.T) {
	issuer := testIssuer(t)
	if rec := sendAs(t, roleRouter(t, issuer, rolePrincipals()), issuer, http.MethodGet, "/teacher/dashboard", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestThePermissionGateRunsAfterAuthenticationAndBeforeTheBody(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	oversized := strings.Repeat(" ", 2<<20) + `{"name":"Lớp"}`
	cases := []struct {
		name, user, body string
		want             int
	}{
		{"anonymous with a bad body", "", "pretzel", http.StatusUnauthorized},
		{"student with a bad body", studentUser, "pretzel", http.StatusForbidden},
		{"student with an oversized body", studentUser, oversized, http.StatusForbidden},
		{"admin with a bad body", adminUser, "pretzel", http.StatusBadRequest},
		{"admin with an oversized body", adminUser, oversized, http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		if rec := sendAs(t, h, issuer, http.MethodPost, "/teacher/classes", c.user, c.body); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

func TestRateLimitsRunBeforeAuthenticationAndThePermissionGate(t *testing.T) {
	issuer := testIssuer(t)
	path := "/app/classes/join"
	for name, c := range map[string]struct {
		user string
		want int
	}{
		"anonymous": {"", http.StatusUnauthorized},
		"teacher":   {teacherUser, http.StatusForbidden},
	} {
		h := roleRouter(t, issuer, rolePrincipals())
		for i := 1; i <= 120; i++ {
			if rec := sendAs(t, h, issuer, http.MethodPost, path, c.user, ""); rec.Code != c.want {
				t.Fatalf("%s request %d: %d, want %d within the budget", name, i, rec.Code, c.want)
			}
		}
		rec := sendAs(t, h, issuer, http.MethodPost, path, c.user, "")
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
			t.Errorf("%s past the budget: %d (Retry-After %q), want 429 before %d", name, rec.Code, rec.Header().Get("Retry-After"), c.want)
		}
	}
}

func isolatedGate(t *testing.T, principals httpx.PrincipalResolver) (http.Handler, map[string]access.Requirement, map[string]struct{}) {
	t.Helper()
	spec := freshSpec(t)
	requirements, err := httpx.PermissionRequirements(spec, "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	open := openPatterns(spec)
	verify := func(raw string) (httpx.Principal, error) {
		userID, epoch, found := strings.Cut(raw, "@")
		if !found {
			return httpx.Principal{}, httpx.ErrUnknownPrincipal
		}
		n, err := strconv.Atoi(epoch)
		return httpx.Principal{UserID: userID, Epoch: n}, err
	}
	passed := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return httpx.RequireAuth(open, verify)(httpx.RequirePermission(requirements, principals)(passed)), requirements, open
}

func throughGate(h http.Handler, pattern, token string) int {
	method, path, _ := strings.Cut(pattern, " ")
	req := httptest.NewRequest(method, strings.NewReplacer("{", "x", "}", "x").Replace(path), nil)
	req.Pattern = pattern
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestEachRoleMeetsExactlyTheOperationsItsPermissionsAllow(t *testing.T) {
	h, requirements, _ := isolatedGate(t, rolePrincipals())
	adminOnly := map[string]bool{"POST /admin/docs-session": true, "DELETE /admin/users/{id}": true}
	for pattern, requirement := range requirements {
		staff := strings.Contains(pattern, " /teacher/") || strings.Contains(pattern, " /admin/")
		student := strings.Contains(pattern, " /app/")
		self := slices.Equal(requirement.Keys(), []access.Key{access.Self})
		for user, want := range map[string]int{
			studentUser: pick(staff, http.StatusForbidden, http.StatusNoContent),
			teacherUser: pick(student || adminOnly[pattern], http.StatusForbidden, http.StatusNoContent),
			adminUser:   pick(student, http.StatusForbidden, http.StatusNoContent),
		} {
			if self {
				want = http.StatusNoContent
			}
			if got := throughGate(h, pattern, user+"@0"); got != want {
				t.Errorf("%s as %s: %d, want %d", pattern, user, got, want)
			}
		}
	}
}

func pick(refused bool, yes, no int) int {
	if refused {
		return yes
	}
	return no
}

func TestTheAssistantMeetsOnlyItsSixRows(t *testing.T) {
	h, requirements, _ := isolatedGate(t, rolePrincipals())
	sixRows := map[access.Key]bool{
		"content.tests.write": true, "content.questions.write": true, "teaching.assignments.write": true,
		"teaching.grading": true, "teaching.attendance": true, "people.students.read": true,
		"self": true, "workspace.teacher": true,
	}
	refused := 0
	for pattern, requirement := range requirements {
		want := http.StatusForbidden
		if slices.ContainsFunc(requirement.Keys(), func(k access.Key) bool { return sixRows[k] }) {
			want = http.StatusNoContent
		}
		if want == http.StatusForbidden {
			refused++
		}
		if got := throughGate(h, pattern, assistantUser+"@0"); got != want {
			t.Errorf("%s as the Assistant: %d, want %d", pattern, got, want)
		}
	}
	if refused != 35 {
		t.Errorf("the Assistant is refused %d operations, want 35: twenty-three staff operations and the twelve /app ones", refused)
	}
}

func TestTheOpenOperationsPassWithAndWithoutAToken(t *testing.T) {
	h, _, open := isolatedGate(t, rolePrincipals())
	for pattern := range open {
		for _, token := range []string{"", studentUser + "@0"} {
			if got := throughGate(h, pattern, token); got != http.StatusNoContent {
				t.Errorf("%s with token %q: %d, want it passed", pattern, token, got)
			}
		}
	}
}

func TestADisabledUserAnUnknownUserAndAStaleTokenGet401(t *testing.T) {
	disabled := builtinPrincipal(teacherUser, access.BuiltinTeacher)
	disabled.Disabled = true
	moved := builtinPrincipal(studentUser, access.BuiltinStudent)
	moved.Epoch = 2
	principals := rolePrincipals().set(disabled).set(moved).set(access.Principal{UserID: ""})
	principals.users["01935000-0000-7000-8000-0000000000ff"] = access.Principal{}
	h, _, _ := isolatedGate(t, principals)
	for name, c := range map[string]struct{ pattern, token string }{
		"disabled":     {"GET /teacher/dashboard", teacherUser + "@0"},
		"stale epoch":  {"GET /app/assignments", studentUser + "@1"},
		"unknown user": {"GET /auth/me", "01935000-0000-7000-8000-0000000000ff@0"},
		"no token":     {"GET /auth/me", ""},
	} {
		if got := throughGate(h, c.pattern, c.token); got != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", name, got)
		}
	}
	if got := throughGate(h, "GET /app/assignments", studentUser+"@2"); got != http.StatusNoContent {
		t.Errorf("a token at the current epoch: %d, want it passed", got)
	}
}

func TestAChangedRoleChangesTheNextAnswer(t *testing.T) {
	principals := rolePrincipals()
	h, _, _ := isolatedGate(t, principals)
	if got := throughGate(h, "POST /teacher/attempts/{id}/grade", teacherUser+"@0"); got != http.StatusNoContent {
		t.Fatalf("a Teacher grading: %d, want it passed", got)
	}
	revoked := builtinPrincipal(teacherUser, access.BuiltinTeacher)
	revoked.Permissions = revoked.Permissions.Without(access.TeachingGrading)
	principals.set(revoked)
	if got := throughGate(h, "POST /teacher/attempts/{id}/grade", teacherUser+"@0"); got != http.StatusForbidden {
		t.Errorf("after the grant is removed: %d, want 403", got)
	}
}

func TestPermissionTreesMatchPaths(t *testing.T) {
	cases := []struct {
		name, path, method string
		value              any
		open               bool
		want               string
	}{
		{name: "an unknown prefix", path: "/staff/things", method: "GET", value: "workspace.teacher", want: "GET /staff/things: lies outside every path tree"},
		{name: "an admin key on the teacher tree", path: "/teacher/rosters", method: "GET", value: "people.users.manage", want: `GET /teacher/rosters: "people.users.manage" does not belong under /teacher/`},
		{name: "a teaching key on the admin tree", path: "/admin/rosters", method: "GET", value: "teaching.grading", want: `GET /admin/rosters: "teaching.grading" does not belong under /admin/`},
		{name: "self on the student tree", path: "/app/rosters", method: "GET", value: "self", want: `GET /app/rosters: "self" does not belong under /app/`},
		{name: "a student key on the auth tree", path: "/auth/rosters", method: "GET", value: "learning.take_tests", want: `GET /auth/rosters: "learning.take_tests" does not belong under /auth/`},
		{name: "a key on the me tree", path: "/me/rosters", method: "GET", value: "teaching.grading", want: `GET /me/rosters: "teaching.grading" does not belong under /me/`},
		{name: "self on the teacher tree", path: "/teacher/rosters", method: "GET", value: "self", want: `GET /teacher/rosters: "self" does not belong under /teacher/`},
		{name: "the admin workspace on the teacher tree", path: "/teacher/rosters", method: "GET", value: "workspace.admin", want: `GET /teacher/rosters: "workspace.admin" does not belong under /teacher/`},
		{name: "a student key on the teacher tree", path: "/teacher/rosters", method: "GET", value: "learning.take_tests", want: `GET /teacher/rosters: "learning.take_tests" does not belong under /teacher/`},
		{name: "one stray key in a list", path: "/teacher/rosters", method: "GET", value: []any{"teaching.grading", "people.roles.manage"}, want: `GET /teacher/rosters: "people.roles.manage" does not belong under /teacher/`},
		{name: "a gated operation on the public tree", path: "/public/rosters", method: "GET", value: "self", want: `GET /public/rosters: "self" does not belong under /public/`},
		{name: "a gated operation on the join tree", path: "/join/rosters", method: "GET", value: "self", want: `GET /join/rosters: "self" does not belong under /join/`},
		{name: "an open operation under an unknown prefix", path: "/staff/open", method: "GET", open: true, want: "GET /staff/open: lies outside every path tree"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := freshSpec(t)
			op := &openapi3.Operation{OperationID: "probe", Responses: openapi3.NewResponses(), Extensions: map[string]any{}}
			if c.open {
				op.Security = openapi3.NewSecurityRequirements()
			} else {
				op.Extensions[httpx.PermissionExtension] = c.value
			}
			item := &openapi3.PathItem{}
			item.SetOperation(c.method, op)
			spec.Paths.Set(c.path, item)
			_, err := httpx.PermissionRequirements(spec, "bearerAuth")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to name %q", err, c.want)
			}
		})
	}
	for _, c := range []struct{ path, value string }{
		{"/me/settings", "self"},
		{"/teacher/rosters", "workspace.teacher"},
		{"/admin/rosters", "workspace.admin"},
		{"/admin/exports", "system.data_export"},
		{"/admin/roles", "people.roles.manage"},
		{"/admin/everything", "scope.all"},
		{"/teacher/students", "people.students.create"},
		{"/app/assignments", "learning.take_tests"},
		{"/auth/sessions", "self"},
	} {
		spec := freshSpec(t)
		op := &openapi3.Operation{OperationID: "probe", Responses: openapi3.NewResponses(), Extensions: map[string]any{httpx.PermissionExtension: c.value}}
		item := &openapi3.PathItem{}
		item.SetOperation("GET", op)
		spec.Paths.Set(c.path, item)
		if _, err := httpx.PermissionRequirements(spec, "bearerAuth"); err != nil {
			t.Errorf("GET %s with %q refused: %v", c.path, c.value, err)
		}
	}
}
