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
	// 403 and not 404: hiding the admin tree would buy nothing -- it is
	// documented and the SPA ships routes for it -- while a 404 sends a
	// teacher whose session downgraded to hunting a broken link.
	if code := errorCode(t, rec); code != "FORBIDDEN" {
		t.Errorf("error code = %q, want FORBIDDEN", code)
	}
}

func TestATeacherReachesTheAdminTree(t *testing.T) {
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	rec := requestAs(t, router, issuer, http.MethodGet, "/admin/dashboard", "admin")
	// Deps.Join and Deps.Auth are nil here, so getting through means 501.
	// Anything but 403 proves the gate opened.
	if rec.Code == http.StatusForbidden {
		t.Fatal("an admin token was refused the admin tree")
	}
}

func TestEveryAdminOperationIsGated(t *testing.T) {
	// Driven by the path, because the path IS the contract's structure (§3).
	// This asserts the derivation actually covers the tree rather than one
	// example of it.
	spec, err := openapi.GetSwagger()
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
	// /app/* belongs to both roles -- a teacher previewing a test is a
	// legitimate caller. Gating it on `student` would lock the teacher out of
	// their own product.
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
	// 401 tells the client to log in; 403 tells it not to bother. Sending 403
	// to someone who simply has no token would strand them.
	issuer := testIssuer(t)
	router := newAuthTestRouter(t, issuer)

	rec := requestAs(t, router, issuer, http.MethodGet, "/admin/dashboard", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
