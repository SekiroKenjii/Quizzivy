package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"quizzivy/internal/core/router"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func newTestRouter(t *testing.T, database router.DB) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := router.New(router.Deps{Principals: newFakePrincipals(), DB: database}, logger, []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatalf("core.NewRouter: %v", err)
	}
	return h
}

func TestHealthzReportsDatabaseReachability(t *testing.T) {
	ok := httptest.NewRecorder()
	newTestRouter(t, fakeDB{}).ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if ok.Code != http.StatusOK {
		t.Errorf("healthy: status = %d, want 200", ok.Code)
	}

	down := httptest.NewRecorder()
	newTestRouter(t, fakeDB{err: errors.New("connection refused")}).
		ServeHTTP(down, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if down.Code != http.StatusServiceUnavailable {
		t.Errorf("unreachable database: status = %d, want 503", down.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(down.Body).Decode(&body)
	if body["database"] != "unreachable" {
		t.Errorf("body = %v, want database:unreachable", body)
	}
}

type countingDB struct {
	pings *int
	err   error
}

func (c countingDB) Ping(context.Context) error {
	*c.pings++
	return c.err
}

func TestLivezAnswersWithoutTouchingTheDatabase(t *testing.T) {
	pings := 0
	rec := httptest.NewRecorder()
	newTestRouter(t, countingDB{pings: &pings, err: errors.New("connection refused")}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 whatever the database says", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body["status"] != "ok" || len(body) != 1 {
		t.Fatalf("body = %v err %v, want only status:ok", body, err)
	}
	if pings != 0 {
		t.Fatalf("liveness pinged the database %d times", pings)
	}
}

func TestHealthRoutesAreRateLimitedPerAddress(t *testing.T) {
	for _, tc := range []struct {
		path   string
		budget int
	}{{"/livez", 30}, {"/healthz", 10}} {
		t.Run(tc.path, func(t *testing.T) {
			handler := newTestRouter(t, fakeDB{})
			for i := 0; i < tc.budget; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("request %d status = %d, want 200 within the budget", i+1, rec.Code)
				}
			}
			over := httptest.NewRecorder()
			handler.ServeHTTP(over, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if over.Code != http.StatusTooManyRequests || over.Header().Get("Retry-After") == "" {
				t.Fatalf("request past the budget: status = %d, Retry-After %q", over.Code, over.Header().Get("Retry-After"))
			}
		})
	}
}

func TestEveryServiceRateLimitNamesARouteTheRouterServes(t *testing.T) {
	handler := newTestRouter(t, fakeDB{})
	for _, pattern := range router.ServiceRateLimits().Patterns() {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q has no method", pattern)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		if rec.Code == http.StatusNotFound {
			t.Errorf("ServiceRateLimits limits %q, which the router does not serve", pattern)
		}
	}
}

func TestUnbuiltOperationReturns501InTheEnvelope(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", 0)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/teacher/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	newAuthTestRouter(t, issuer).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 (a 404 would look like a routing bug)", rec.Code)
	}
	var env struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("response is not the error envelope: %v", err)
	}
	if env.Error.RequestID == "" {
		t.Error("requestId is empty; §9's copyable error ID depends on it")
	}
	if rec.Header().Get("X-Request-Id") != env.Error.RequestID {
		t.Error("the header and the envelope must carry the same request id")
	}
}

func TestSeparateClientsGetSeparateBudgets(t *testing.T) {
	handler := newTestRouter(t, fakeDB{})
	exhaust := func(ip string) {
		for i := 0; i < 121; i++ {
			req := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
			req.RemoteAddr = ip + ":1111"
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}
	}
	exhaust("198.51.100.1")

	req := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
	req.RemoteAddr = "198.51.100.2:2222"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Error("one client's exhaustion must not block another")
	}
}

var theFormerTeachingPathsUnderAdmin = []string{
	"DELETE /admin/assignments/{id}",
	"DELETE /admin/classes/{id}",
	"DELETE /admin/classes/{id}/join-code",
	"DELETE /admin/classes/{id}/members/{userId}",
	"DELETE /admin/media/{id}",
	"DELETE /admin/question-groups/{id}",
	"DELETE /admin/questions/{id}",
	"DELETE /admin/students/{id}",
	"DELETE /admin/tests/{id}",
	"DELETE /admin/tests/{id}/versions/{version}",
	"GET /admin/assignments",
	"GET /admin/assignments/{id}",
	"GET /admin/assignments/{id}/answers",
	"GET /admin/assignments/{id}/attempts",
	"GET /admin/attempts",
	"GET /admin/attempts/{id}",
	"GET /admin/attempts/{id}/events",
	"GET /admin/classes/{id}",
	"GET /admin/classes/{id}/members",
	"GET /admin/dashboard",
	"GET /admin/imports",
	"GET /admin/imports/capabilities",
	"GET /admin/imports/limits",
	"GET /admin/imports/{id}",
	"GET /admin/imports/{id}/review",
	"GET /admin/imports/{id}/source",
	"GET /admin/imports/{id}/sources/{sourceId}/download",
	"GET /admin/media",
	"GET /admin/question-groups",
	"GET /admin/question-groups/{id}",
	"GET /admin/questions",
	"GET /admin/questions/{id}",
	"GET /admin/students",
	"GET /admin/students/{id}",
	"GET /admin/tests",
	"GET /admin/tests/{id}",
	"GET /admin/tests/{id}/preview",
	"GET /admin/tests/{id}/versions",
	"PATCH /admin/assignments/{id}",
	"PATCH /admin/attempts/{id}/note",
	"PATCH /admin/classes/{id}",
	"PATCH /admin/question-groups/{id}/archive",
	"PATCH /admin/questions/{id}",
	"PATCH /admin/students/{id}",
	"PATCH /admin/tests/{id}",
	"POST /admin/assignments",
	"POST /admin/assignments/{id}/reopen",
	"POST /admin/attempts/{id}/extend",
	"POST /admin/attempts/{id}/finish-grading",
	"POST /admin/attempts/{id}/flag",
	"POST /admin/attempts/{id}/grade",
	"POST /admin/attempts/{id}/reset",
	"POST /admin/attempts/{id}/void",
	"POST /admin/classes/{id}/join-code",
	"POST /admin/classes/{id}/members",
	"POST /admin/imports",
	"POST /admin/imports/{id}/cancel",
	"POST /admin/imports/{id}/commit",
	"POST /admin/imports/{id}/process",
	"POST /admin/imports/{id}/review/adopt",
	"POST /admin/imports/{id}/sources",
	"POST /admin/media",
	"POST /admin/question-groups",
	"POST /admin/question-groups/{id}/copy",
	"POST /admin/questions",
	"POST /admin/questions/tags",
	"POST /admin/questions/{id}/duplicate",
	"POST /admin/students",
	"POST /admin/students/{id}/reset-password",
	"POST /admin/tests",
	"POST /admin/tests/{id}/duplicate",
	"POST /admin/tests/{id}/publish",
	"POST /admin/tests/{id}/versions/{version}/current",
	"POST /admin/tests/{id}/versions/{version}/draft",
	"PUT /admin/imports/{id}/review",
	"PUT /admin/question-groups/{id}",
}

func TestAFormerTeachingPathUnderAdminIsNotFound(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	declared := map[string]bool{}
	eachOperation(freshSpec(t), func(pattern string, _ *openapi3.Operation) { declared[pattern] = true })
	undeclared := envelopeOf(t, sendAs(t, h, issuer, http.MethodGet, "/admin/no-such", adminUser, ""))
	id := "01935000-0000-7000-8000-0000000000e1"
	for _, pattern := range theFormerTeachingPathsUnderAdmin {
		if declared[pattern] {
			t.Errorf("%s is in the contract again", pattern)
			continue
		}
		method, path, _ := strings.Cut(pattern, " ")
		path = strings.NewReplacer("{id}", id, "{userId}", id, "{sourceId}", id, "{version}", "1").Replace(path)
		for _, user := range []string{"", adminUser} {
			var rec *httptest.ResponseRecorder
			if user == "" {
				rec = send(h, method, path, nil)
			} else {
				rec = sendAs(t, h, issuer, method, path, user, "")
			}
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s as %q = %d, want 404", method, path, user, rec.Code)
				continue
			}
			if env := envelopeOf(t, rec); env.Error.Code != undeclared.Error.Code || env.Error.Message != undeclared.Error.Message {
				t.Errorf("%s %s as %q: answer = %s %q, want the undeclared path's %s %q", method, path, user,
					env.Error.Code, env.Error.Message, undeclared.Error.Code, undeclared.Error.Message)
			}
		}
	}
}
