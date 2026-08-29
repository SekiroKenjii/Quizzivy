package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
)

// Authentication says who you are; this says whether the /admin tree is yours.
// Until the first real admin endpoint existed, every /admin route was a 501
// stub and the distinction cost nothing. It costs a great deal now: the first
// one hands out join codes.

func requestAs(t *testing.T, router http.Handler, issuer interface {
	Issue(userID, role string) (string, error)
}, method, path, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if role != "" {
		token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", role)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAStudentCannotReachTheAdminTree(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	rec := requestAs(t, router, issuer, http.MethodGet, "/admin/dashboard", "student")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if code := errorCode(t, rec); code != "FORBIDDEN" {
		t.Errorf("error code = %q, want FORBIDDEN", code)
	}
}

func TestATeacherReachesTheAdminTree(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	rec := requestAs(t, router, issuer, http.MethodGet, "/admin/dashboard", "admin")
	if rec.Code == http.StatusForbidden {
		t.Fatal("an admin token was refused the admin tree")
	}
}

func TestEveryAdminOperationIsGated(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}

	var admin, ungated []string
	for path, item := range spec.Paths.Map() {
		if !strings.HasPrefix(path, httpx.AdminPathPrefix) {
			continue
		}
		for method := range item.Operations() {
			pattern := method + " " + path
			admin = append(admin, pattern)
			if !httpx.IsAdminPattern(pattern) {
				ungated = append(ungated, pattern)
			}
		}
	}

	if len(admin) < 25 {
		t.Fatalf("only %d admin operations found; the derivation is looking at the wrong thing", len(admin))
	}
	if len(ungated) > 0 {
		t.Errorf("admin operations not recognised by the gate: %v", ungated)
	}
}

func TestTheStudentTreeIsNotGatedByRole(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	for _, role := range []string{"student", "admin"} {
		rec := requestAs(t, router, issuer, http.MethodGet, "/app/assignments", role)
		if rec.Code == http.StatusForbidden {
			t.Errorf("role %q was refused the student tree", role)
		}
	}
}

func TestAnAnonymousCallerToTheAdminTreeGetsAuthenticationNotAuthorization(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	rec := requestAs(t, router, issuer, http.MethodGet, "/admin/dashboard", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
