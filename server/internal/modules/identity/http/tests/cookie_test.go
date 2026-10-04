package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"quizzivy/internal/modules/identity/application/model"
	"strings"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/cqrs"
)

type fakeAuth struct {
	refreshToken string
	presented    string
	logoutErr    error
}

func (f *fakeAuth) app() *application.Application {
	login := func(context.Context, command.Login) (model.Session, error) {
		return model.Session{AccessToken: "access", ExpiresIn: 900, RefreshToken: f.refreshToken}, nil
	}
	refresh := func(_ context.Context, cmd command.Refresh) (model.RefreshResult, error) {
		f.presented = cmd.Token
		return model.RefreshResult{AccessToken: "access", ExpiresIn: 900, RefreshToken: "next"}, nil
	}
	logout := func(_ context.Context, cmd command.Logout) (cqrs.Nothing, error) {
		f.presented = cmd.Token
		return cqrs.Nothing{}, f.logoutErr
	}
	return &application.Application{Commands: application.Commands{
		Login:   cqrs.HandlerFunc[command.Login, model.Session](login),
		Refresh: cqrs.HandlerFunc[command.Refresh, model.RefreshResult](refresh),
		Logout:  cqrs.HandlerFunc[command.Logout, cqrs.Nothing](logout),
	}}
}

func loginCookie(t *testing.T, ttl time.Duration, secure bool) *http.Cookie {
	t.Helper()
	h := identityhttp.NewIdentity((&fakeAuth{refreshToken: "opaque-token-value"}).app(), ttl, secure, nil)
	resp, err := h.Login(context.Background(), openapi.LoginRequestObject{
		Body: &openapi.LoginJSONRequestBody{Email: openapi_types.Email("a@example.com"), Password: "mật-khẩu"},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	ok, isOK := resp.(openapi.Login200JSONResponse)
	if !isOK || ok.Headers.SetCookie == nil {
		t.Fatalf("Login answered %T without a Set-Cookie header", resp)
	}
	c, err := http.ParseSetCookie(*ok.Headers.SetCookie)
	if err != nil {
		t.Fatalf("Set-Cookie %q does not parse: %v", *ok.Headers.SetCookie, err)
	}
	return c
}

func TestRefreshCookieCarriesExactlyTheDocumentedAttributes(t *testing.T) {
	c := loginCookie(t, 30*24*time.Hour, true)

	if c.Name != "quizzivy_refresh" || c.Value != "opaque-token-value" {
		t.Errorf("cookie = %s=%s", c.Name, c.Value)
	}
	if c.Path != "/auth" || c.MaxAge != 2592000 || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("path %q max-age %d samesite %v", c.Path, c.MaxAge, c.SameSite)
	}
	if !c.HttpOnly {
		t.Error("HttpOnly is missing: script could read the refresh token")
	}
	if !c.Secure {
		t.Error("Secure is missing: the refresh token would travel over plain http")
	}
	if c.Domain != "" {
		t.Errorf("Domain = %q, want empty (host-only)", c.Domain)
	}
}

func TestRefreshCookieSecureFlagFollowsConfiguration(t *testing.T) {
	if loginCookie(t, time.Hour, false).Secure {
		t.Error("Secure set when configuration disabled it")
	}
	if !loginCookie(t, time.Hour, true).Secure {
		t.Error("Secure not set when configuration enabled it")
	}
}

func TestLogoutClearsTheCookieItReplaces(t *testing.T) {
	live := loginCookie(t, time.Hour, true)
	h := identityhttp.NewIdentity((&fakeAuth{}).app(), time.Hour, true, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: live.Name, Value: "the-token"})

	var resp openapi.LogoutResponseObject
	var err error
	identityhttp.WithRefreshCookie(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		resp, err = h.Logout(r.Context(), openapi.LogoutRequestObject{})
	})).ServeHTTP(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
	rec := httptest.NewRecorder()
	if err := resp.VisitLogoutResponse(rec); err != nil || rec.Code != http.StatusNoContent {
		t.Fatalf("Logout rendered %d: %v", rec.Code, err)
	}
	cookies := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		cookies[c.Name] = c
	}
	cleared := cookies[live.Name]
	if cleared == nil {
		t.Fatalf("Logout did not clear %s: %v", live.Name, rec.Header().Values("Set-Cookie"))
	}
	if cleared.Path != live.Path || cleared.Domain != live.Domain {
		t.Errorf("cleared cookie identity (%s,%q) != live (%s,%q)", cleared.Path, cleared.Domain, live.Path, live.Domain)
	}
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("cleared cookie still lives: value %q max-age %d", cleared.Value, cleared.MaxAge)
	}
	if !cleared.HttpOnly || !cleared.Secure {
		t.Error("cleared cookie dropped HttpOnly/Secure; some browsers refuse the overwrite")
	}
	docs := cookies["quizzivy_docs"]
	if docs == nil || docs.Path != "/docs" || docs.Value != "" || docs.MaxAge >= 0 || !docs.HttpOnly || !docs.Secure || docs.SameSite != http.SameSiteStrictMode {
		t.Errorf("signing out left the API reference session open: %+v", docs)
	}
}

func TestAFailedLogoutRendersItsErrorAfterTheClears(t *testing.T) {
	revoke := errors.New("revoke failed")
	h := identityhttp.NewIdentity((&fakeAuth{logoutErr: revoke}).app(), time.Hour, true, nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "quizzivy_refresh", Value: "the-token"})

	var resp openapi.LogoutResponseObject
	var err error
	identityhttp.WithRefreshCookie(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		resp, err = h.Logout(r.Context(), openapi.LogoutRequestObject{})
	})).ServeHTTP(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Logout: %v, want the failure carried by the response so the clears are sent with it", err)
	}
	if resp == nil {
		t.Fatal("Logout answered nothing for a revoke that failed")
	}
	rec := httptest.NewRecorder()
	if got := resp.VisitLogoutResponse(rec); !errors.Is(got, revoke) {
		t.Fatalf("the response rendered %v, want the revoke's error for the router's 500", got)
	}
	cookies := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		cookies[c.Name] = c
	}
	cleared := cookies["quizzivy_refresh"]
	if cleared == nil {
		t.Fatalf("a failed logout did not clear quizzivy_refresh: %v", rec.Header().Values("Set-Cookie"))
	}
	if cleared.Value != "" || cleared.Path != "/auth" || cleared.MaxAge >= 0 {
		t.Errorf("cleared cookie still lives: value %q path %q max-age %d", cleared.Value, cleared.Path, cleared.MaxAge)
	}
	if !cleared.HttpOnly || !cleared.Secure {
		t.Error("cleared cookie dropped HttpOnly/Secure; some browsers refuse the overwrite")
	}
	docs := cookies["quizzivy_docs"]
	if docs == nil || docs.Path != "/docs" || docs.Value != "" || docs.MaxAge >= 0 {
		t.Errorf("a failed logout left the API reference session open: %+v", docs)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the response wrote %q, want nothing: the router writes the 500", rec.Body.String())
	}
}

func TestTheRefusedLogoutClearFollowsTheSecureSetting(t *testing.T) {
	for _, secure := range []bool{true, false} {
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: "quizzivy_refresh", Value: "x"})
		rec := httptest.NewRecorder()
		identityhttp.NewIdentity(nil, time.Hour, secure, nil).ClearSessionOnRefusedLogout(rec, req)

		cookies := map[string]*http.Cookie{}
		for _, c := range rec.Result().Cookies() {
			cookies[c.Name] = c
		}
		cleared := cookies["quizzivy_refresh"]
		if cleared == nil {
			t.Fatalf("secure=%t: the refused logout did not clear quizzivy_refresh: %v", secure, rec.Header().Values("Set-Cookie"))
		}
		if cleared.Secure != secure {
			t.Errorf("secure=%t: the clear's Secure is %t; a browser refuses a clear that does not match the cookie it replaces", secure, cleared.Secure)
		}
		if !cleared.HttpOnly || cleared.Path != "/auth" {
			t.Errorf("secure=%t: the clear is %+v, want HttpOnly on /auth", secure, cleared)
		}
		if cookies["quizzivy_docs"] == nil {
			t.Errorf("secure=%t: the refused logout did not clear quizzivy_docs: %v", secure, rec.Header().Values("Set-Cookie"))
		}
	}
}

func TestCookieNameMatchesTheContract(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	scheme, ok := spec.Components.SecuritySchemes["refreshCookie"]
	if !ok || scheme.Value == nil {
		t.Fatal("api/openapi.yaml no longer defines a `refreshCookie` security scheme")
	}
	if scheme.Value.In != "cookie" {
		t.Errorf("refreshCookie is declared in %q, want cookie", scheme.Value.In)
	}
	if got := loginCookie(t, time.Hour, true).Name; scheme.Value.Name != got {
		t.Errorf("contract cookie name %q != the cookie the API sets %q", scheme.Value.Name, got)
	}
}

func TestMiddlewareLiftsTheCookieAndToleratesItsAbsence(t *testing.T) {
	name := loginCookie(t, time.Hour, true).Name
	presented := func(cookie *http.Cookie) string {
		fake := &fakeAuth{}
		h := identityhttp.NewIdentity(fake.app(), time.Hour, true, nil)
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		identityhttp.WithRefreshCookie(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, _ = h.RefreshSession(r.Context(), openapi.RefreshSessionRequestObject{})
		})).ServeHTTP(httptest.NewRecorder(), req)
		return fake.presented
	}

	if got := presented(&http.Cookie{Name: name, Value: "the-token"}); got != "the-token" {
		t.Errorf("token presented = %q, want the-token", got)
	}
	if got := presented(nil); got != "" {
		t.Errorf("token presented without a cookie = %q, want empty", got)
	}
	if got := presented(&http.Cookie{Name: name, Value: ""}); strings.TrimSpace(got) != "" {
		t.Errorf("empty cookie presented %q, want empty", got)
	}
}
