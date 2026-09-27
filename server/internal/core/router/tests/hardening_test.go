package router_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestBodyLimitPrecedesJSONValidation(t *testing.T) {
	for _, length := range []int64{-1, 2 << 20} {
		t.Run(strconv.FormatInt(length, 10), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(strings.Repeat(" ", 2<<20)+`{"email":"student@example.com","password":"a-password"}`))
			req.ContentLength = length
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newTestRouter(t, fakeDB{}).ServeHTTP(rec, req)
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413; body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSecurityHeadersOnSuccessFailureAndPreflight(t *testing.T) {
	private := map[string]bool{"/auth/login": true, "/app/assignments": true, "/teacher/dashboard": true, "/admin/docs-session": true}
	for _, path := range []string{"/livez", "/healthz", "/app/assignments", "/auth/login", "/teacher/dashboard", "/admin/docs-session", "/missing"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestRouter(t, fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			for _, header := range []string{"X-Content-Type-Options", "Content-Security-Policy", "Strict-Transport-Security", "Referrer-Policy", "Permissions-Policy"} {
				if rec.Header().Get(header) == "" {
					t.Errorf("missing %s", header)
				}
			}
			if private[path] && rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s is cacheable: Cache-Control %q", path, rec.Header().Get("Cache-Control"))
			}
		})
	}
	window := underWay()
	window.over.Store(true)
	status := httptest.NewRecorder()
	gatedRouter(t, window, testIssuer(t)).ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/public/status", nil))
	if got := status.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Errorf("/public/status Cache-Control = %q, want its own public, max-age=30", got)
	}
	request := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	request.Header.Set("Origin", "https://app.quizzivy.com")
	request.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	newTestRouter(t, fakeDB{}).ServeHTTP(rec, request)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("preflight bypassed security headers")
	}
}
