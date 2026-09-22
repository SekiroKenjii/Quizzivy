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
	for _, path := range []string{"/healthz", "/app/assignments", "/auth/login", "/missing"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestRouter(t, fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			for _, header := range []string{"X-Content-Type-Options", "Content-Security-Policy", "Strict-Transport-Security", "Referrer-Policy", "Permissions-Policy"} {
				if rec.Header().Get(header) == "" {
					t.Errorf("missing %s", header)
				}
			}
			if strings.HasPrefix(path, "/auth/") && rec.Header().Get("Cache-Control") != "no-store" {
				t.Error("auth response is cacheable")
			}
		})
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
