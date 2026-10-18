package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

var theOnesOpenToATemporaryPassword = []string{"GET /auth/me", "POST /auth/change-password"}

func temporaryPrincipals() *fakePrincipals {
	temporary := func(p access.Principal) access.Principal {
		p.UserID += "-tmp"
		p.MustChangePassword = true
		return p
	}
	return rolePrincipals().
		set(temporary(builtinPrincipal(adminUser, access.BuiltinAdmin))).
		set(temporary(builtinPrincipal(teacherUser, access.BuiltinTeacher))).
		set(temporary(builtinPrincipal(studentUser, access.BuiltinStudent)))
}

func answerOf(h http.Handler, pattern, token string) (int, string) {
	method, path, _ := strings.Cut(pattern, " ")
	req := httptest.NewRequest(method, strings.NewReplacer("{", "x", "}", "x").Replace(path), nil)
	req.Pattern = pattern
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body.Error.Code
}

func TestOnlyReadingTheAccountAndChangingThePasswordAreOpenToATemporaryPassword(t *testing.T) {
	requirements, err := httpx.PermissionRequirements(freshSpec(t), "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	var marked []string
	for pattern, requirement := range requirements {
		if requirement.AllowsTemporaryPassword() {
			marked = append(marked, pattern)
		}
	}
	sort.Strings(marked)
	if !slices.Equal(marked, theOnesOpenToATemporaryPassword) {
		t.Errorf("operations open to a temporary password = %v, want %v", marked, theOnesOpenToATemporaryPassword)
	}
}

func TestAnAccountOnATemporaryPasswordMeetsNothingElseWhateverItsRole(t *testing.T) {
	h, requirements, _ := isolatedGate(t, temporaryPrincipals())
	for pattern, requirement := range requirements {
		for _, user := range []string{adminUser, teacherUser, studentUser} {
			status, code := answerOf(h, pattern, user+"-tmp@0")
			if requirement.AllowsTemporaryPassword() {
				if status != http.StatusNoContent {
					t.Errorf("%s as %s on a temporary password: %d, want it passed", pattern, user, status)
				}
				continue
			}
			if status != http.StatusForbidden || code != "PASSWORD_CHANGE_REQUIRED" {
				t.Errorf("%s as %s on a temporary password: %d %s, want 403 PASSWORD_CHANGE_REQUIRED whether or not the role could use it", pattern, user, status, code)
			}
		}
	}
}

func TestTheSameAccountsPassAsBeforeOnceThePasswordIsChanged(t *testing.T) {
	h, requirements, _ := isolatedGate(t, rolePrincipals())
	for pattern := range requirements {
		if status, code := answerOf(h, pattern, adminUser+"@0"); code == "PASSWORD_CHANGE_REQUIRED" {
			t.Errorf("%s as an Admin with no temporary password: %d %s", pattern, status, code)
		}
	}
}

func TestTheTemporaryPasswordMarkIsRefusedOnAnythingButASelfOperationOrAFlag(t *testing.T) {
	for name, c := range map[string]struct {
		path, method string
		value        any
		want         string
	}{
		"on a permissioned operation": {"/teacher/classes", "GET", true, "x-allows-temporary-password is allowed only with the pseudo-key"},
		"on an admin operation":       {"/admin/users", "GET", true, "x-allows-temporary-password is allowed only with the pseudo-key"},
		"as a word":                   {"/auth/me", "GET", "yes", `declares yes, which is not true or false`},
		"as a number":                 {"/auth/me", "GET", 1, `declares 1, which is not true or false`},
	} {
		t.Run(name, func(t *testing.T) {
			spec := freshSpec(t)
			op := spec.Paths.Find(c.path).GetOperation(c.method)
			if op.Extensions == nil {
				op.Extensions = map[string]any{}
			}
			op.Extensions[httpx.TemporaryPasswordExtension] = c.value

			_, err := httpx.PermissionRequirements(spec, "bearerAuth")

			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to name %q", err, c.want)
			}
		})
	}

	spec := freshSpec(t)
	spec.Paths.Find("/teacher/classes").GetOperation("GET").Extensions[httpx.TemporaryPasswordExtension] = false
	requirements, err := httpx.PermissionRequirements(spec, "bearerAuth")
	if err != nil || requirements["GET /teacher/classes"].AllowsTemporaryPassword() {
		t.Errorf("false on a permissioned operation: %v, allows %v, want it accepted and not allowing", err, requirements["GET /teacher/classes"].AllowsTemporaryPassword())
	}
}
