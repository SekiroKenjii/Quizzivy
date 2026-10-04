package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
)

const (
	pathNotFoundVI     = "Không tìm thấy đường dẫn."
	pathNotFoundEN     = "The path was not found."
	methodNotAllowedVI = "Phương thức này không dùng được cho đường dẫn này."
	methodNotAllowedEN = "This method is not allowed for this path."
)

type unroutedEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

func envelopeOf(t *testing.T, rec *httptest.ResponseRecorder) unroutedEnvelope {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var env unroutedEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("response is not the error envelope: %v", err)
	}
	return env
}

func contractMethods(t *testing.T, path string) string {
	t.Helper()
	item := freshSpec(t).Paths.Find(path)
	if item == nil {
		t.Fatalf("the contract has no path %s", path)
	}
	var methods []string
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if item.GetOperation(method) != nil {
			methods = append(methods, method)
		}
	}
	return strings.Join(methods, ", ")
}

func TestAnUnknownPathAnswersTheEnvelope(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	for acceptLanguage, want := range map[string]string{"": pathNotFoundVI, "en": pathNotFoundEN} {
		rec := sendIn(t, h, http.MethodGet, "/no-such-path", "", acceptLanguage, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("Accept-Language %q: status = %d, want 404", acceptLanguage, rec.Code)
		}
		env := envelopeOf(t, rec)
		if env.Error.Code != "NOT_FOUND" || env.Error.Message != want {
			t.Errorf("Accept-Language %q: answer = %s %q, want NOT_FOUND %q", acceptLanguage, env.Error.Code, env.Error.Message, want)
		}
		if env.Error.RequestID == "" || env.Error.RequestID != rec.Header().Get("X-Request-Id") {
			t.Errorf("Accept-Language %q: requestId = %q, X-Request-Id = %q, want one id in both",
				acceptLanguage, env.Error.RequestID, rec.Header().Get("X-Request-Id"))
		}
	}
}

func TestAnUnknownPathIsTheSameUnderEveryMethod(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/no-such-path"},
		{http.MethodDelete, "/no-such-path"},
		{http.MethodHead, "/no-such-path"},
		{http.MethodGet, "/auth/me/extra"},
		{http.MethodGet, "/auth/me/"},
	} {
		rec := send(h, tc.method, tc.path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", tc.method, tc.path, rec.Code)
			continue
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("%s %s: Content-Type = %q, want application/json", tc.method, tc.path, got)
		}
		if got, sent := rec.Header()["Allow"]; sent {
			t.Errorf("%s %s: Allow = %q on a 404, want none", tc.method, tc.path, got)
		}
		if tc.method == http.MethodHead {
			continue
		}
		if env := envelopeOf(t, rec); env.Error.Code != "NOT_FOUND" || env.Error.Message != pathNotFoundVI {
			t.Errorf("%s %s: answer = %s %q, want NOT_FOUND %q", tc.method, tc.path, env.Error.Code, env.Error.Message, pathNotFoundVI)
		}
	}
}

func TestAnUnknownPathSpendsNoBudget(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	for i := 1; i <= 300; i++ {
		if rec := send(h, http.MethodGet, "/no-such-path", nil); rec.Code != http.StatusNotFound {
			t.Fatalf("request %d = %d, want 404 every time: an unknown path has no bucket to spend", i, rec.Code)
		}
	}
	if rec := send(h, http.MethodGet, "/livez", nil); rec.Code != http.StatusOK {
		t.Errorf("GET /livez after 300 unknown paths from the same address = %d, want 200", rec.Code)
	}
}

func TestAKnownPathUnderAWrongMethodAnswers405(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	for _, tc := range []struct{ method, path, allow string }{
		{http.MethodDelete, "/public/status", "GET, HEAD"},
		{http.MethodGet, "/auth/login", "POST"},
		{http.MethodPut, saveAnswersPath, "PATCH"},
		{http.MethodDelete, "/livez", "GET, HEAD"},
	} {
		for acceptLanguage, want := range map[string]string{"": methodNotAllowedVI, "en": methodNotAllowedEN} {
			rec := sendIn(t, h, tc.method, tc.path, "", acceptLanguage, "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", tc.method, tc.path, rec.Code)
				continue
			}
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Errorf("%s %s: Allow = %q, want %q", tc.method, tc.path, got, tc.allow)
			}
			env := envelopeOf(t, rec)
			if env.Error.Code != "METHOD_NOT_ALLOWED" || env.Error.Message != want {
				t.Errorf("%s %s, Accept-Language %q: answer = %s %q, want METHOD_NOT_ALLOWED %q",
					tc.method, tc.path, acceptLanguage, env.Error.Code, env.Error.Message, want)
			}
			if env.Error.RequestID == "" || env.Error.RequestID != rec.Header().Get("X-Request-Id") {
				t.Errorf("%s %s: requestId = %q, X-Request-Id = %q, want one id in both",
					tc.method, tc.path, env.Error.RequestID, rec.Header().Get("X-Request-Id"))
			}
		}
	}
}

func TestHeadOnAnOpenGetIsTheGetWithoutABody(t *testing.T) {
	window := underWay()
	window.over.Store(true)
	h := gatedRouter(t, window, testIssuer(t))

	get := send(h, http.MethodGet, "/public/status", nil)
	head := send(h, http.MethodHead, "/public/status", nil)

	if get.Code != http.StatusOK || get.Body.Len() == 0 {
		t.Fatalf("GET /public/status = %d with %d bytes, want 200 with a body", get.Code, get.Body.Len())
	}
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD /public/status = %d, want 200: %s", head.Code, head.Body.String())
	}
	for _, name := range []string{"Content-Type", "Cache-Control"} {
		if got, want := head.Header().Get(name), get.Header().Get(name); got != want || want == "" {
			t.Errorf("HEAD /public/status: %s = %q, want the GET's %q", name, got, want)
		}
	}
	if head.Body.Len() != 0 {
		t.Errorf("HEAD /public/status wrote a body: %q", head.Body.String())
	}
}

func TestHeadOnAGetThatNeedsATokenIs405(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	allow := contractMethods(t, "/auth/me")
	if !strings.HasPrefix(allow, http.MethodGet) {
		t.Fatalf("the contract serves /auth/me under %q, want a GET for this test to mean anything", allow)
	}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"without a token":    sendAs(t, h, issuer, http.MethodHead, "/auth/me", "", ""),
		"with a valid token": sendAs(t, h, issuer, http.MethodHead, "/auth/me", adminUser, ""),
	} {
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("HEAD /auth/me %s = %d, want 405", name, rec.Code)
			continue
		}
		if got := rec.Header().Get("Allow"); got != allow {
			t.Errorf("HEAD /auth/me %s: Allow = %q, want the contract's %q", name, got, allow)
		}
		if env := envelopeOf(t, rec); env.Error.Code != "METHOD_NOT_ALLOWED" {
			t.Errorf("HEAD /auth/me %s: code = %q, want METHOD_NOT_ALLOWED", name, env.Error.Code)
		}
	}
}

func TestHeadOnTheServiceRoutesIsUnchanged(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	for _, path := range []string{"/livez", "/healthz"} {
		if rec := send(h, http.MethodHead, path, nil); rec.Code != http.StatusOK {
			t.Errorf("HEAD %s = %d, want 200", path, rec.Code)
		}
	}
}

func TestDuringAWindowTheUnroutedAnswersHoldTheirPlace(t *testing.T) {
	window := underWay()
	h := gatedRouter(t, window, testIssuer(t))

	unknown := send(h, http.MethodGet, "/no-such-path", nil)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("GET /no-such-path during a window = %d, want 404", unknown.Code)
	}
	if env := envelopeOf(t, unknown); env.Error.Code != "NOT_FOUND" {
		t.Errorf("GET /no-such-path during a window: code = %q, want NOT_FOUND", env.Error.Code)
	}
	if n := window.asked.Load(); n != 0 {
		t.Errorf("the gate asked about the window %d times for a path no route serves", n)
	}

	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodDelete, "/auth/me", http.StatusServiceUnavailable},
		{http.MethodHead, "/public/status", http.StatusOK},
		{http.MethodHead, "/auth/me", http.StatusServiceUnavailable},
	} {
		if rec := send(h, tc.method, tc.path, nil); rec.Code != tc.status {
			t.Errorf("%s %s during a window = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
		}
	}
}

func TestTheUnroutedAnswersCarryCORS(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/no-such-path", http.StatusNotFound},
		{http.MethodDelete, "/auth/me", http.StatusMethodNotAllowed},
	} {
		rec := send(h, tc.method, tc.path, map[string]string{"Origin": allowedOrigin})
		if rec.Code != tc.status {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.status)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
			t.Errorf("%s %s: Access-Control-Allow-Origin = %q; without it the SPA cannot read the %d", tc.method, tc.path, got, tc.status)
		}
	}
}

func TestMethodNotAllowedIsInTheContract(t *testing.T) {
	if !openapi.METHODNOTALLOWED.Valid() {
		t.Error("METHOD_NOT_ALLOWED is not a member of the contract's ErrorCode")
	}
	if string(openapi.METHODNOTALLOWED) != string(httpx.CodeMethodNotAllowed) {
		t.Errorf("httpx.CodeMethodNotAllowed = %q, want the contract's %q", httpx.CodeMethodNotAllowed, openapi.METHODNOTALLOWED)
	}
}

func TestTheLegacyAliasKeepsTheSameAnswers(t *testing.T) {
	h := roleRouter(t, testIssuer(t), rolePrincipals())

	head := send(h, http.MethodHead, "/admin/tests", nil)
	if head.Code != http.StatusMethodNotAllowed {
		t.Errorf("HEAD /admin/tests = %d, want 405", head.Code)
	}
	if got, want := head.Header().Get("Allow"), contractMethods(t, "/teacher/tests"); got != want {
		t.Errorf("HEAD /admin/tests: Allow = %q, want the contract's %q for /teacher/tests", got, want)
	}

	unknown := send(h, http.MethodGet, "/admin/no-such", nil)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("GET /admin/no-such = %d, want 404", unknown.Code)
	}
	if env := envelopeOf(t, unknown); env.Error.Code != "NOT_FOUND" || env.Error.Message != pathNotFoundVI {
		t.Errorf("GET /admin/no-such: answer = %s %q, want NOT_FOUND %q", env.Error.Code, env.Error.Message, pathNotFoundVI)
	}
}

func TestAPathTheMuxCleansIsRedirectedAsBefore(t *testing.T) {
	window := underWay()
	window.over.Store(true)
	h := gatedRouter(t, window, testIssuer(t))
	for _, tc := range []struct{ method, path, location string }{
		{http.MethodGet, "//auth/me", "/auth/me"},
		{http.MethodHead, "//auth/me", "/auth/me"},
		{http.MethodDelete, "//auth/me", "/auth/me"},
		{http.MethodHead, "//public/status", "/public/status"},
		{http.MethodGet, "//no-such-path", "/no-such-path"},
		{http.MethodGet, "/a/../b", "/b"},
		{http.MethodPost, "/a/../b", "/b"},
	} {
		rec := send(h, tc.method, tc.path, nil)
		if rec.Code != http.StatusTemporaryRedirect || rec.Header().Get("Location") != tc.location {
			t.Errorf("%s %s = %d to %q, want the mux's 307 to %q", tc.method, tc.path, rec.Code, rec.Header().Get("Location"), tc.location)
		}
	}

	window.over.Store(false)
	asked := window.asked.Load()
	if rec := send(h, http.MethodGet, "//no-such-path", nil); rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("GET //no-such-path during a window = %d, want the mux's 307", rec.Code)
	}
	if n := window.asked.Load() - asked; n != 0 {
		t.Errorf("the gate asked about the window %d times for a path that cleans to one no route serves", n)
	}
	if rec := send(h, http.MethodDelete, "//auth/me", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("DELETE //auth/me during a window = %d, want 503", rec.Code)
	}
}
