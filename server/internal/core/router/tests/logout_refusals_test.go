package router_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/internal/core/router"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/cqrs"
)

func logoutRouter(t *testing.T, revoke error) http.Handler {
	t.Helper()
	logout := func(context.Context, command.Logout) (cqrs.Nothing, error) {
		return cqrs.Nothing{}, revoke
	}
	app := &application.Application{Commands: application.Commands{
		Logout: cqrs.HandlerFunc[command.Logout, cqrs.Nothing](logout),
	}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := router.New(router.Deps{
		Principals: newFakePrincipals(),
		DB:         fakeDB{},
		Tokens:     testIssuer(t),
		Modules:    router.Modules{Identity: identityhttp.NewIdentity(app, time.Hour, true, nil)},
	}, logger, []string{allowedOrigin}, "")
	if err != nil {
		t.Fatalf("router.New: %v", err)
	}
	return h
}

func firstRefusal(t *testing.T, h http.Handler, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	for i := 1; i <= 300; i++ {
		rec := send(h, http.MethodPost, "/auth/logout", header)
		if rec.Code != http.StatusTooManyRequests {
			continue
		}
		if i <= 100 {
			t.Fatalf("logout %d from one address was refused by the limiter; a classroom's sign-outs must pass", i)
		}
		return rec
	}
	t.Fatal("300 logouts from one address and the limiter refused none")
	return nil
}

func assertSessionCleared(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Values("Set-Cookie"); len(got) != 2 {
		t.Fatalf("the %d carries Set-Cookie %v, want the two clears; the next person on this device is signed in", rec.Code, got)
	}
	cleared := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		cleared[c.Name] = c
	}
	refresh, ok := cleared["quizzivy_refresh"]
	switch {
	case !ok:
		t.Fatalf("the %d does not clear quizzivy_refresh: %v", rec.Code, rec.Header().Values("Set-Cookie"))
	case refresh.Value != "", refresh.Path != "/auth", refresh.MaxAge >= 0, !refresh.HttpOnly, !refresh.Secure:
		t.Errorf("quizzivy_refresh = %+v, want an empty HttpOnly Secure cookie on /auth with a negative Max-Age", refresh)
	}
	docs, ok := cleared["quizzivy_docs"]
	switch {
	case !ok:
		t.Fatalf("the %d does not clear quizzivy_docs: %v", rec.Code, rec.Header().Values("Set-Cookie"))
	case docs.Value != "", docs.Path != "/docs", docs.MaxAge >= 0:
		t.Errorf("quizzivy_docs = %+v, want an empty cookie on /docs with a negative Max-Age", docs)
	}
}

func TestALogoutTheLimiterRefusesStillClearsTheSessionCookies(t *testing.T) {
	rec := firstRefusal(t, logoutRouter(t, nil), map[string]string{"Cookie": "quizzivy_refresh=anything"})

	if rec.Header().Get("Retry-After") == "" {
		t.Error("the 429 carries no Retry-After")
	}
	assertSessionCleared(t, rec)
	if code := errorCode(t, rec); code != "RATE_LIMITED" {
		t.Errorf("code = %q, want RATE_LIMITED", code)
	}
}

func TestALogoutOverTheLimitWithNoSessionClearsNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"a logout with no cookie", nil},
		{"a logout with an empty cookie", map[string]string{"Cookie": "quizzivy_refresh="}},
	} {
		rec := firstRefusal(t, logoutRouter(t, nil), tc.header)
		if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
			t.Errorf("%s carries Set-Cookie %v on the 429, want none: another site could sign a user out", tc.name, got)
		}
	}
}

func TestALogoutWhoseRevokeFailsStillClearsTheSessionCookies(t *testing.T) {
	h := logoutRouter(t, errors.New("revoke failed"))
	rec := send(h, http.MethodPost, "/auth/logout", map[string]string{"Cookie": "quizzivy_refresh=anything"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	assertSessionCleared(t, rec)
	if code := errorCode(t, rec); code != "INTERNAL" {
		t.Errorf("code = %q, want INTERNAL", code)
	}
}

func TestALogoutThatSucceedsSendsEachClearOnce(t *testing.T) {
	rec := send(logoutRouter(t, nil), http.MethodPost, "/auth/logout", map[string]string{"Cookie": "quizzivy_refresh=anything"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Values("Set-Cookie"); len(got) != 2 {
		t.Errorf("the 204 carries Set-Cookie %v, want each clear once", got)
	}
}
