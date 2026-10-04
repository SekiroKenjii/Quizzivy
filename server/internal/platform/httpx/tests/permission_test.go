package httpx_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

type resolver map[string]access.Principal

func (r resolver) Resolve(_ context.Context, userID string) (access.Principal, error) {
	if userID == "broken" {
		return access.Principal{}, errors.New("database unreachable")
	}
	p, ok := r[userID]
	if !ok {
		return access.Principal{}, httpx.ErrUnknownPrincipal
	}
	return p, nil
}

var principals = resolver{
	"grader":  {UserID: "grader", Permissions: access.NewSet(access.TeachingGrading)},
	"student": {UserID: "student", Permissions: access.NewSet(access.LearningTakeTests)},
	"auditor": {UserID: "auditor", Permissions: access.NewSet(access.SystemAuditRead)},
	"nobody":  {UserID: "nobody", Permissions: access.Set{}},
}

var requirements = map[string]access.Requirement{
	"GET /self":    access.AnyOf(access.Self),
	"GET /teacher": access.AnyOf(access.WorkspaceTeacher),
	"GET /admin":   access.AnyOf(access.WorkspaceAdmin),
	"GET /review":  access.AnyOf(access.TeachingGrading, access.TeachingAttemptsIntervene),
	"GET /app":     access.AnyOf(access.LearningTakeTests),
}

func gate(t *testing.T, seen *access.Principal) http.Handler {
	t.Helper()
	return gateBefore(principals, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := httpx.PrincipalFromContext(r.Context()); ok && seen != nil {
			*seen = p.Access
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

func gateBefore(resolved httpx.PrincipalResolver, next http.Handler) http.Handler {
	open := map[string]struct{}{"POST /open": {}}
	verify := func(raw string) (httpx.Principal, error) {
		userID, epoch, _ := strings.Cut(raw, "@")
		n, err := strconv.Atoi(epoch)
		return httpx.Principal{UserID: userID, Epoch: n}, err
	}
	return httpx.RequireAuth(open, verify)(httpx.RequirePermission(requirements, resolved)(next))
}

func call(h http.Handler, pattern, token string) int {
	method, path, _ := strings.Cut(pattern, " ")
	req := httptest.NewRequest(method, path, nil)
	req.Pattern = pattern
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestAnOpenRoutePassesWithAndWithoutAToken(t *testing.T) {
	h := gate(t, nil)
	for _, token := range []string{"", "student@0", "unknown@0"} {
		if got := call(h, "POST /open", token); got != http.StatusNoContent {
			t.Errorf("open route with %q: %d, want it passed", token, got)
		}
	}
}

func TestEachPseudoKeyAndAnyOfList(t *testing.T) {
	h := gate(t, nil)
	cases := []struct {
		pattern string
		passes  []string
		refused []string
	}{
		{"GET /self", []string{"grader", "student", "auditor", "nobody"}, nil},
		{"GET /teacher", []string{"grader"}, []string{"student", "auditor", "nobody"}},
		{"GET /admin", []string{"auditor"}, []string{"grader", "student", "nobody"}},
		{"GET /review", []string{"grader"}, []string{"student", "auditor", "nobody"}},
		{"GET /app", []string{"student"}, []string{"grader", "auditor", "nobody"}},
	}
	for _, c := range cases {
		for _, user := range c.passes {
			if got := call(h, c.pattern, user+"@0"); got != http.StatusNoContent {
				t.Errorf("%s as %s: %d, want it passed", c.pattern, user, got)
			}
		}
		for _, user := range c.refused {
			if got := call(h, c.pattern, user+"@0"); got != http.StatusForbidden {
				t.Errorf("%s as %s: %d, want 403", c.pattern, user, got)
			}
		}
	}
}

func TestTheResolvedPrincipalJoinsTheContext(t *testing.T) {
	var seen access.Principal
	if got := call(gate(t, &seen), "GET /review", "grader@0"); got != http.StatusNoContent {
		t.Fatalf("status %d", got)
	}
	if seen.UserID != "grader" || !slices.Equal(seen.Permissions.Keys(), []access.Key{access.TeachingGrading}) {
		t.Errorf("the context holds %+v", seen)
	}
}

func TestAGatedRouteRefusesTheUnauthenticatedAndTheUnknown(t *testing.T) {
	h := gate(t, nil)
	for name, token := range map[string]string{"no token": "", "unknown user": "ghost@0"} {
		if got := call(h, "GET /self", token); got != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", name, got)
		}
	}
	if got := call(h, "GET /self", "broken@0"); got != http.StatusInternalServerError {
		t.Errorf("a failing resolver: %d, want 500", got)
	}
}

func TestADisabledUserOrAStaleEpochGets401WithAChallenge(t *testing.T) {
	stale := resolver{"lan": {UserID: "lan", Epoch: 3, Permissions: access.NewSet(access.LearningTakeTests)}, "minh": {UserID: "minh", Disabled: true, Permissions: access.NewSet(access.LearningTakeTests)}}
	verify := func(raw string) (httpx.Principal, error) {
		userID, epoch, _ := strings.Cut(raw, "@")
		n, err := strconv.Atoi(epoch)
		return httpx.Principal{UserID: userID, Epoch: n}, err
	}
	h := httpx.RequireAuth(nil, verify)(httpx.RequirePermission(requirements, stale)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	for name, token := range map[string]string{"stale epoch": "lan@2", "disabled": "minh@0"} {
		req := httptest.NewRequest(http.MethodGet, "/app", nil)
		req.Pattern = "GET /app"
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("%s: %d with challenge %q, want 401 with a Bearer challenge", name, rec.Code, rec.Header().Get("WWW-Authenticate"))
		}
	}
	if got := call(httpx.RequireAuth(nil, verify)(httpx.RequirePermission(requirements, stale)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))), "GET /app", "lan@3"); got != http.StatusNoContent {
		t.Errorf("the current epoch: %d, want it passed", got)
	}
}
