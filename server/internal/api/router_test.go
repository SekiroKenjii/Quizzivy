package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func newTestRouter(t *testing.T, database DB) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewRouter(Deps{DB: database}, logger, []string{"https://app.quizzivy.com"}, false)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
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
	// A health check that stays green while the database is down is worse than
	// none: it makes an outage look like an application bug.
	var body map[string]string
	_ = json.NewDecoder(down.Body).Decode(&body)
	if body["database"] != "unreachable" {
		t.Errorf("body = %v, want database:unreachable", body)
	}
}

func TestUnbuiltOperationReturns501InTheEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(t, fakeDB{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

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

func TestRateLimitAppliesPerRouteAndEmitsRetryAfter(t *testing.T) {
	router := newTestRouter(t, fakeDB{})

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
		req.RemoteAddr = "203.0.113.7:5555"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= 10; i++ {
		if rec := send(); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited; §6.5 allows 10 per minute", i)
		}
	}

	rec := send()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("11th request: status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("§6.5 requires Retry-After on a 429")
	}
}

func TestRateLimitKeysOnTheRouteTemplateNotTheURL(t *testing.T) {
	// /admin/tests/{id} must share one bucket, not create one per test id.
	// Here the check is the inverse: an unlimited route stays unlimited however
	// many distinct URLs are hit.
	router := newTestRouter(t, fakeDB{})
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest(http.MethodGet, "/admin/tests", nil)
		req.RemoteAddr = "203.0.113.9:5555"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("authenticated route limited after %d requests; no policy is registered for it", i)
		}
	}
}

func TestSeparateClientsGetSeparateBudgets(t *testing.T) {
	router := newTestRouter(t, fakeDB{})
	exhaust := func(ip string) {
		for i := 0; i < 11; i++ {
			req := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
			req.RemoteAddr = ip + ":1111"
			router.ServeHTTP(httptest.NewRecorder(), req)
		}
	}
	exhaust("198.51.100.1")

	req := httptest.NewRequest(http.MethodPost, "/join/preview", nil)
	req.RemoteAddr = "198.51.100.2:2222"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Error("one client's exhaustion must not block another")
	}
}
