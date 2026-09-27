package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/core/router"
	availabilityapp "quizzivy/internal/modules/availability/application"
	availabilitydomain "quizzivy/internal/modules/availability/domain"
	availabilityhttp "quizzivy/internal/modules/availability/http"
	identitytoken "quizzivy/internal/modules/identity/application/token"
)

const allowedOrigin = "https://app.quizzivy.com"

type maintenanceWindow struct {
	startsAt, endsAt time.Time
	over             atomic.Bool
	asked            atomic.Int32
}

func underWay() *maintenanceWindow {
	now := time.Now()
	return &maintenanceWindow{startsAt: now.Add(-10 * time.Minute), endsAt: now.Add(90 * time.Minute)}
}

func (w *maintenanceWindow) ActiveWindow(context.Context) (time.Time, time.Time, bool) {
	w.asked.Add(1)
	return w.startsAt, w.endsAt, !w.over.Load()
}

func (w *maintenanceWindow) Next(context.Context, time.Time) (*availabilitydomain.Window, error) {
	return &availabilitydomain.Window{StartsAt: w.startsAt, EndsAt: w.endsAt}, nil
}

func gatedRouter(t *testing.T, window *maintenanceWindow, issuer *identitytoken.Issuer) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := router.New(router.Deps{
		Principals:  newFakePrincipals(),
		DB:          fakeDB{},
		Tokens:      issuer,
		Maintenance: window,
		Modules:     router.Modules{Availability: availabilityhttp.NewAvailability(availabilityapp.New(window, nil))},
	}, logger, []string{allowedOrigin}, "")
	if err != nil {
		t.Fatalf("router.New: %v", err)
	}
	return h
}

func send(h http.Handler, method, path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "203.0.113.40:5555"
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type maintenanceEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
		Details   struct {
			StartsAt time.Time `json:"startsAt"`
			EndsAt   time.Time `json:"endsAt"`
		} `json:"details"`
	} `json:"error"`
}

func TestDuringAWindowTheAPIAnswers503TheSPACanRead(t *testing.T) {
	window := underWay()
	rec := send(gatedRouter(t, window, testIssuer(t)), http.MethodGet, "/auth/me", map[string]string{"Origin": allowedOrigin})

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q; without it the SPA cannot read the 503", got)
	}
	retry, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retry < 89*60 || retry > 90*60 {
		t.Errorf("Retry-After = %q, want the seconds until the window ends", rec.Header().Get("Retry-After"))
	}
	var env maintenanceEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("not the error envelope: %v", err)
	}
	switch {
	case env.Error.Code != "MAINTENANCE":
		t.Errorf("code = %q, want MAINTENANCE", env.Error.Code)
	case !env.Error.Details.StartsAt.Equal(window.startsAt.Truncate(time.Second)),
		!env.Error.Details.EndsAt.Equal(window.endsAt.Truncate(time.Second)):
		t.Errorf("details = %+v, want the window", env.Error.Details)
	case env.Error.Message != "Quizzivy đang được cập nhật. Vui lòng quay lại khi cập nhật xong.":
		t.Errorf("message = %q, want Vietnamese by default", env.Error.Message)
	case env.Error.RequestID == "" || rec.Header().Get("X-Request-Id") != env.Error.RequestID:
		t.Error("the 503 carries no request id")
	}
}

func TestTheMaintenanceMessageFollowsTheBrowser(t *testing.T) {
	rec := send(gatedRouter(t, underWay(), testIssuer(t)), http.MethodGet, "/auth/me", map[string]string{"Accept-Language": "en-US,en;q=0.9"})
	var env maintenanceEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Message != "Quizzivy is being updated. Please come back when the update ends." {
		t.Errorf("message = %q, want English", env.Error.Message)
	}
}

func TestHealthAndStatusAnswerDuringAWindow(t *testing.T) {
	h := gatedRouter(t, underWay(), testIssuer(t))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/livez"},
		{http.MethodHead, "/livez"},
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/public/status"},
	} {
		if rec := send(h, tc.method, tc.path, nil); rec.Code != http.StatusOK {
			t.Errorf("%s %s = %d during a window, want 200", tc.method, tc.path, rec.Code)
		}
	}
	var body struct {
		Maintenance struct {
			Active bool `json:"active"`
		} `json:"maintenance"`
	}
	rec := send(h, http.MethodGet, "/public/status", nil)
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || !body.Maintenance.Active {
		t.Errorf("/public/status = %s, want the active window", rec.Body.String())
	}
}

func TestNoRouteIsExemptBeyondTheThree(t *testing.T) {
	h := gatedRouter(t, underWay(), testIssuer(t))
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/auth/login"},
		{http.MethodPost, "/auth/refresh"},
		{http.MethodPost, "/join/preview"},
		{http.MethodPost, "/app/attempts/01935000-0000-7000-8000-0000000000b1/events"},
		{http.MethodPut, "/app/attempts/01935000-0000-7000-8000-0000000000b1/answers"},
		{http.MethodGet, "/admin/tests"},
		{http.MethodGet, "/docs"},
		{http.MethodPost, "/public/status"},
	} {
		if rec := send(h, tc.method, tc.path, nil); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s = %d during a window, want 503", tc.method, tc.path, rec.Code)
		}
	}
}

func TestAPathNoRouteServesIsNeverAskedAboutAWindow(t *testing.T) {
	window := underWay()
	h := gatedRouter(t, window, testIssuer(t))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/.env"},
		{http.MethodGet, "/wp-login.php"},
		{http.MethodPost, "/xmlrpc.php"},
		{http.MethodGet, "/auth/me/extra"},
	} {
		if rec := send(h, tc.method, tc.path, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d during a window, want the router's 404", tc.method, tc.path, rec.Code)
		}
	}
	if n := window.asked.Load(); n != 0 {
		t.Errorf("the gate asked about the window %d times for paths no route serves; each ask can wake the database", n)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodDelete, "/auth/me"},
		{http.MethodGet, "/auth/login"},
		{http.MethodGet, "/app/attempts/01935000-0000-7000-8000-0000000000b1/answers"},
		{http.MethodGet, "/admin/media/01935000-0000-7000-8000-0000000000c1"},
	} {
		if rec := send(h, tc.method, tc.path, nil); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s = %d during a window, want 503: a route's path stays gated under any method", tc.method, tc.path, rec.Code)
		}
	}
}

func TestAnExpiredTokenDuringAWindowIs503Not401(t *testing.T) {
	issuer := testIssuer(t)
	issuer.SetClock(func() time.Time { return time.Now().Add(-time.Hour) })
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "student", 0)
	if err != nil {
		t.Fatal(err)
	}
	issuer.SetClock(time.Now)
	header := map[string]string{"Authorization": "Bearer " + token}

	if rec := send(newAuthTestRouter(t, issuer), http.MethodGet, "/auth/me", header); rec.Code != http.StatusUnauthorized {
		t.Fatalf("outside a window the expired token got %d, want 401", rec.Code)
	}
	if rec := send(gatedRouter(t, underWay(), issuer), http.MethodGet, "/auth/me", header); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("during a window the expired token got %d, want 503 so the SPA does not refresh and sign out", rec.Code)
	}
}

func TestTheSameRequestPassesOnceTheWindowEnds(t *testing.T) {
	window := underWay()
	h := gatedRouter(t, window, testIssuer(t))
	if rec := send(h, http.MethodGet, "/auth/me", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("during the window: %d, want 503", rec.Code)
	}
	window.over.Store(true)
	if rec := send(h, http.MethodGet, "/auth/me", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("after the window: %d, want the route's own 401", rec.Code)
	}
}

func TestTheGateSitsInsideCORSAndBeforeRateLimits(t *testing.T) {
	h := gatedRouter(t, underWay(), testIssuer(t))

	preflight := send(h, http.MethodOptions, "/auth/login", map[string]string{
		"Origin":                        allowedOrigin,
		"Access-Control-Request-Method": http.MethodPost,
	})
	if preflight.Code == http.StatusServiceUnavailable || preflight.Header().Get("Access-Control-Allow-Origin") != allowedOrigin {
		t.Errorf("preflight = %d, allow-origin %q; CORS must answer it before the gate",
			preflight.Code, preflight.Header().Get("Access-Control-Allow-Origin"))
	}
	for i := 1; i <= 15; i++ {
		if rec := send(h, http.MethodPost, "/join/preview", nil); rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("join preview %d = %d during a window, want 503 before the limiter counts it", i, rec.Code)
		}
	}
}
