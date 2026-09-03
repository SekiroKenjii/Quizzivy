package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/auth"
	"quizzivy/internal/httpx"
)

// Authentication is driven by api/openapi.yaml, not by a list in Go: an
// operation inherits the document's top-level `security` unless it overrides
// it. That makes "forgot to protect the new endpoint" impossible by
// construction, and makes the small set of exceptions worth pinning.

// theOpenSix is every operation reachable without an access token. It is short
// on purpose. If this list grows, someone opened an endpoint, and that should
// take an argument rather than a diff nobody reads.
var theOpenSix = []string{
	"POST /app/attempts/{id}/events", // D-03's beacon path; see above
	"POST /auth/google",              // sign-in, by definition pre-session
	"POST /auth/login",               // ditto
	"POST /auth/logout",              // authenticated by the refresh COOKIE instead
	"POST /auth/refresh",             // ditto -- the access token is expected to be dead
	"POST /join/preview",             // §6.5's public join surface
}

func TestOnlySixOperationsAreReachableWithoutAnAccessToken(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}

	open := httpx.OpenRoutes(spec, "bearerAuth")
	got := make([]string, 0, len(open))
	for route := range open {
		got = append(got, route)
	}
	sort.Strings(got)

	if strings.Join(got, "\n") != strings.Join(theOpenSix, "\n") {
		t.Errorf("open routes changed.\n got: %v\nwant: %v", got, theOpenSix)
	}

	total := 0
	for _, item := range spec.Paths.Map() {
		total += len(item.Operations())
	}
	if total-len(got) < 50 {
		t.Errorf("only %d of %d operations require authentication; the derivation "+
			"is probably matching the wrong scheme name", total-len(got), total)
	}
}

func newAuthTestRouter(t *testing.T, issuer *auth.TokenIssuer) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewRouter(Deps{DB: fakeDB{}, Tokens: issuer}, logger,
		[]string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return h
}

func testIssuer(t *testing.T) *auth.TokenIssuer {
	t.Helper()
	issuer, err := auth.NewTokenIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return issuer
}

func TestAProtectedRouteRefusesAnAnonymousCaller(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	// RFC 7235 requires the challenge on a 401.
	if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, want a Bearer challenge", got)
	}

	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if code := body["error"]["code"]; code != "UNAUTHORIZED" {
		t.Errorf("error code = %v, want UNAUTHORIZED", code)
	}
}

func TestRejectedCredentialsAllLookTheSame(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	expired, err := auth.NewTokenIssuer([]byte(strings.Repeat("k", 32)), time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	expiredToken, err := expired.Issue("01935000-0000-7000-8000-0000000000a1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := auth.NewTokenIssuer([]byte(strings.Repeat("x", 32)), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	foreignToken, err := foreign.Issue("01935000-0000-7000-8000-0000000000a1", "admin")
	if err != nil {
		t.Fatal(err)
	}

	for name, header := range map[string]string{
		"no header":         "",
		"empty bearer":      "Bearer ",
		"not bearer":        "Basic dXNlcjpwYXNz",
		"scheme only":       "Bearer",
		"garbage token":     "Bearer not-a-jwt",
		"expired token":     "Bearer " + expiredToken,
		"signed by another": "Bearer " + foreignToken,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
	_ = issuer
}

func TestALowercaseBearerSchemeIsAccepted(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("a lowercase `bearer` scheme was rejected")
	}
}

func TestAnOpenRouteIsReachableWithoutTheHeader(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))
	req := httptest.NewRequest(http.MethodPost, "/auth/login",
		strings.NewReader(`{"email":"a@b.com","password":"whatever1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("/auth/login demanded an access token; sign-in is now impossible")
	}
}

func TestHealthzIsNotBehindAuthentication(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAMisconfiguredVerifierRefusesEveryoneRatherThanNobody(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router, err := NewRouter(Deps{DB: fakeDB{}}, logger, []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer anything-at-all")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
