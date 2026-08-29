package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quizzivy/gen/openapi"
)

// The refresh cookie's attributes ARE the session security model (§5.2). Each
// one is load-bearing and none of them is visible in a passing feature test:
// drop Secure and the token travels in the clear, drop HttpOnly and script can
// read it, add Domain and every subdomain receives it.

func attrs(c *http.Cookie) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(c.String(), "; ")[1:] {
		k, v, found := strings.Cut(part, "=")
		if !found {
			v = ""
		}
		out[strings.ToLower(k)] = v
	}
	return out
}

func TestRefreshCookieCarriesExactlyTheDocumentedAttributes(t *testing.T) {
	c := refreshCookie("opaque-token-value", 30*24*time.Hour, true)

	if c.Name != "quizzivy_refresh" {
		t.Errorf("name = %q", c.Name)
	}
	if c.Value != "opaque-token-value" {
		t.Errorf("value = %q", c.Value)
	}

	got := attrs(c)
	for k, want := range map[string]string{
		"path":     "/auth",
		"max-age":  "2592000",
		"samesite": "Lax",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
	if _, ok := got["httponly"]; !ok {
		t.Error("HttpOnly is missing: script could read the refresh token")
	}
	if _, ok := got["secure"]; !ok {
		t.Error("Secure is missing: the refresh token would travel over plain http")
	}
}

func TestRefreshCookieHasNoDomainAttribute(t *testing.T) {
	c := refreshCookie("t", time.Hour, true)
	if c.Domain != "" {
		t.Fatalf("Domain = %q, want empty (host-only)", c.Domain)
	}
	if strings.Contains(strings.ToLower(c.String()), "domain=") {
		t.Fatalf("rendered cookie carries a Domain attribute: %s", c.String())
	}
}

func TestRefreshCookieSecureFlagFollowsConfiguration(t *testing.T) {
	// The one environment where Secure is off is plain-http localhost.
	if strings.Contains(refreshCookie("t", time.Hour, false).String(), "Secure") {
		t.Error("Secure set when configuration disabled it")
	}
	if !strings.Contains(refreshCookie("t", time.Hour, true).String(), "Secure") {
		t.Error("Secure not set when configuration enabled it")
	}
}

func TestClearingCookieMatchesTheOneItReplaces(t *testing.T) {
	live := refreshCookie("t", time.Hour, true)
	cleared := clearRefreshCookie(true)

	if cleared.Name != live.Name || cleared.Path != live.Path || cleared.Domain != live.Domain {
		t.Errorf("cleared cookie identity (%s,%s,%q) != live (%s,%s,%q)",
			cleared.Name, cleared.Path, cleared.Domain, live.Name, live.Path, live.Domain)
	}
	if cleared.Value != "" {
		t.Errorf("cleared cookie still carries a value: %q", cleared.Value)
	}
	if !strings.Contains(cleared.String(), "Max-Age=0") {
		t.Errorf("cleared cookie does not expire immediately: %s", cleared.String())
	}
	if !cleared.HttpOnly || !cleared.Secure {
		t.Error("cleared cookie dropped HttpOnly/Secure; some browsers refuse the overwrite")
	}
}

func TestCookieNameMatchesTheContract(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}
	scheme, ok := spec.Components.SecuritySchemes["refreshCookie"]
	if !ok || scheme.Value == nil {
		t.Fatal("api/openapi.yaml no longer defines a `refreshCookie` security scheme")
	}
	if scheme.Value.In != "cookie" {
		t.Errorf("refreshCookie is declared in %q, want cookie", scheme.Value.In)
	}
	if scheme.Value.Name != refreshCookieName {
		t.Errorf("contract cookie name %q != Go constant %q", scheme.Value.Name, refreshCookieName)
	}
}

func TestMiddlewareLiftsTheCookieAndToleratesItsAbsence(t *testing.T) {
	var seen string
	h := WithRefreshCookie(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = refreshTokenFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "the-token"})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "the-token" {
		t.Errorf("token from context = %q, want the-token", seen)
	}
	seen = "unset"
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/auth/refresh", nil))
	if seen != "" {
		t.Errorf("token from context = %q, want empty", seen)
	}

	// A present-but-empty cookie must not read as a token either.
	seen = "unset"
	req = httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: ""})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "" {
		t.Errorf("empty cookie produced token %q, want empty", seen)
	}
}
