package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

func temporaryGate() http.Handler {
	reqs := map[string]access.Requirement{
		"GET /me":      access.AnyOf(access.Self).AllowingTemporaryPassword(),
		"GET /self":    access.AnyOf(access.Self),
		"GET /teacher": access.AnyOf(access.WorkspaceTeacher),
	}
	res := resolver{
		"fresh":    {UserID: "fresh", Permissions: access.NewSet(access.TeachingGrading), MustChangePassword: true},
		"disabled": {UserID: "disabled", Permissions: access.NewSet(access.TeachingGrading), MustChangePassword: true, Disabled: true},
		"stale":    {UserID: "stale", Permissions: access.NewSet(access.TeachingGrading), MustChangePassword: true, Epoch: 3},
		"changed":  {UserID: "changed", Permissions: access.NewSet(access.TeachingGrading)},
		"student":  {UserID: "student", Permissions: access.NewSet(access.LearningTakeTests), MustChangePassword: true},
	}
	verify := func(raw string) (httpx.Principal, error) {
		userID, epoch, _ := strings.Cut(raw, "@")
		n, err := strconv.Atoi(epoch)
		return httpx.Principal{UserID: userID, Epoch: n}, err
	}
	passed := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return httpx.RequireAuth(nil, verify)(httpx.RequirePermission(reqs, res)(passed))
}

func temporaryCall(h http.Handler, pattern, token, language string) (int, map[string]any) {
	method, path, _ := strings.Cut(pattern, " ")
	req := httptest.NewRequest(method, path, nil)
	req.Pattern = pattern
	req.Header.Set("Authorization", "Bearer "+token)
	if language != "" {
		req.Header.Set("Accept-Language", language)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestATemporaryPasswordIsAnsweredBeforeAnyPermissionIsWeighed(t *testing.T) {
	h := temporaryGate()
	for _, c := range []struct {
		pattern, user string
	}{
		{"GET /self", "fresh"},
		{"GET /teacher", "fresh"},
		{"GET /teacher", "student"},
	} {
		status, body := temporaryCall(h, c.pattern, c.user+"@0", "en")
		envelope, _ := body["error"].(map[string]any)
		if status != http.StatusForbidden || envelope["code"] != "PASSWORD_CHANGE_REQUIRED" {
			t.Errorf("%s as %s: %d %v, want 403 PASSWORD_CHANGE_REQUIRED even where the role would be refused anyway", c.pattern, c.user, status, body)
		}
	}
}

func TestTheOperationsThatAllowATemporaryPasswordPassForItAndTheMessageIsInTheCallersLanguage(t *testing.T) {
	h := temporaryGate()
	if status, _ := temporaryCall(h, "GET /me", "fresh@0", ""); status != http.StatusNoContent {
		t.Errorf("GET /me on a temporary password: %d, want it passed", status)
	}
	for language, want := range map[string]string{
		"vi": "Bạn cần đổi mật khẩu tạm thời trước khi tiếp tục.",
		"en": "Change your temporary password before you continue.",
	} {
		_, body := temporaryCall(h, "GET /self", "fresh@0", language)
		envelope, _ := body["error"].(map[string]any)
		if envelope["message"] != want {
			t.Errorf("message in %s = %v, want %q", language, envelope["message"], want)
		}
	}
}

func TestADisabledOrSignedOutAccountOnATemporaryPasswordGets401NotThePasswordPrompt(t *testing.T) {
	h := temporaryGate()
	for _, token := range []string{"disabled@0", "stale@1"} {
		if status, _ := temporaryCall(h, "GET /self", token, ""); status != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401 so the refresh settles the session first", token, status)
		}
	}
}

func TestOnceThePasswordIsChangedTheAccountMeetsItsPermissionsAgain(t *testing.T) {
	h := temporaryGate()
	if status, _ := temporaryCall(h, "GET /teacher", "changed@0", ""); status != http.StatusNoContent {
		t.Errorf("after the change: %d, want it passed", status)
	}
}
