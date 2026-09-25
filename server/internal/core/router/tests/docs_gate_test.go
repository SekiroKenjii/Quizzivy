package router_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	identityhttp "quizzivy/internal/modules/identity/http"
)

type docsHarness struct {
	handler http.Handler
	access  *identitytoken.Issuer
	docs    *identitytoken.Issuer
}

func newDocsRouter(t *testing.T, public bool) docsHarness {
	t.Helper()
	access := testIssuer(t)
	docs, err := identitytoken.NewDocsIssuer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := router.New(router.Deps{
		DB:         fakeDB{},
		Tokens:     access,
		Docs:       docs,
		DocsPublic: public,
		Modules:    router.Modules{Identity: identityhttp.NewIdentity(nil, time.Hour, true, docs)},
	}, logger, []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return docsHarness{handler: h, access: access, docs: docs}
}

func (h docsHarness) get(t *testing.T, path string, prepare func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if prepare != nil {
		prepare(req)
	}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

func withDocsCookie(value string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "quizzivy_docs", Value: value}) }
}

func (h docsHarness) docsToken(t *testing.T, role string) string {
	t.Helper()
	raw, err := h.docs.Issue("01935000-0000-7000-8000-0000000000d1", role)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, header := range []string{"X-Content-Type-Options", "Content-Security-Policy", "Strict-Transport-Security", "Referrer-Policy"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("missing %s", header)
		}
	}
}

func TestTheDocsRefuseACallerWithoutADocsSession(t *testing.T) {
	h := newDocsRouter(t, false)
	accessToken, err := h.access.Issue("01935000-0000-7000-8000-0000000000d1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	expired, err := identitytoken.NewDocsIssuer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	expired.SetClock(func() time.Time { return time.Now().Add(-time.Hour) })
	stale, err := expired.Issue("01935000-0000-7000-8000-0000000000d1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	valid := h.docsToken(t, "admin")
	cases := map[string]func(*http.Request){
		"no cookie":                     nil,
		"an access token as a bearer":   func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+accessToken) },
		"an access token as the cookie": withDocsCookie(accessToken),
		"an expired docs token":         withDocsCookie(stale),
		"a tampered docs token":         withDocsCookie(spliced(valid, h.docsToken(t, "student"))),
	}
	for _, path := range []string{"/docs", "/docs/openapi.json"} {
		for name, prepare := range cases {
			t.Run(path+" with "+name, func(t *testing.T) {
				rec := h.get(t, path, prepare)
				if rec.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401", rec.Code)
				}
				if strings.Contains(rec.Body.String(), "<html") || strings.Contains(rec.Body.String(), "openapi") {
					t.Fatal("a refused request leaked the page or the contract")
				}
				assertSecurityHeaders(t, rec)
			})
		}
	}
}

func TestTheDocsRefuseANonAdminDocsSession(t *testing.T) {
	h := newDocsRouter(t, false)
	rec := h.get(t, "/docs", withDocsCookie(h.docsToken(t, "student")))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	assertSecurityHeaders(t, rec)
}

func TestTheDocsOpenForAnAdminDocsSession(t *testing.T) {
	h := newDocsRouter(t, false)
	for _, path := range []string{"/docs", "/docs/openapi.json"} {
		rec := h.get(t, path, withDocsCookie(h.docsToken(t, "admin")))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
		assertSecurityHeaders(t, rec)
	}
}

func TestLocalDevelopmentCanOpenTheDocsWithoutASession(t *testing.T) {
	rec := newDocsRouter(t, true).get(t, "/docs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with DOCS_PUBLIC", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "script-src 'sha256-") {
		t.Fatal("the public page lost its script policy")
	}
}

func TestTheDocsAreRateLimitedPerAddress(t *testing.T) {
	for _, path := range []string{"/docs", "/docs/openapi.json"} {
		t.Run(path, func(t *testing.T) {
			h := newDocsRouter(t, false)
			for i := 0; i < 20; i++ {
				if rec := h.get(t, path, nil); rec.Code != http.StatusUnauthorized {
					t.Fatalf("request %d status = %d, want 401 within the budget", i+1, rec.Code)
				}
			}
			if rec := h.get(t, path, nil); rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request past the budget status = %d, want 429", rec.Code)
			}
		})
	}
}

func spliced(signedFor, claimsFrom string) string {
	signature := signedFor[strings.LastIndex(signedFor, ".")+1:]
	return claimsFrom[:strings.LastIndex(claimsFrom, ".")+1] + signature
}

func TestOnlyAnAdminCanOpenADocsSession(t *testing.T) {
	h := newDocsRouter(t, false)
	post := func(role string) *httptest.ResponseRecorder {
		raw, err := h.access.Issue("01935000-0000-7000-8000-0000000000d1", role)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/docs-session", nil)
		req.Header.Set("Authorization", "Bearer "+raw)
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		return rec
	}
	if rec := post("student"); rec.Code != http.StatusForbidden {
		t.Fatalf("student status = %d, want 403", rec.Code)
	}
	rec := post("admin")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("admin status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want exactly the docs cookie", cookies)
	}
	c := cookies[0]
	if c.Name != "quizzivy_docs" || c.Path != "/docs" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.MaxAge != 900 {
		t.Fatalf("cookie = %+v, want quizzivy_docs; Path=/docs; HttpOnly; Secure; SameSite=Strict; Max-Age=900", c)
	}
	if opened := h.get(t, "/docs", withDocsCookie(c.Value)); opened.Code != http.StatusOK {
		t.Fatalf("the minted cookie did not open the docs: %d", opened.Code)
	}
}
