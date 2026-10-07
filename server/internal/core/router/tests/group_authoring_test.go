package router_test

import (
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
	"strings"
	"testing"
)

func TestGroupAuthoringRemainsTeacherOnlyBeforeReadingContent(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for path, item := range spec.Paths.Map() {
		if !strings.HasPrefix(path, "/teacher/question-groups") {
			continue
		}
		path = strings.ReplaceAll(path, "{id}", "01935000-0000-7000-8000-000000000001")
		for method := range item.Operations() {
			target := path
			if method == http.MethodDelete {
				target += "?expectedRevision=1"
			}
			for _, user := range []string{"", studentUser} {
				response := sendAs(t, handler, issuer, method, target, user, "")
				want := http.StatusForbidden
				if user == "" {
					want = http.StatusUnauthorized
				}
				if response.Code != want {
					t.Fatalf("%s %s (%s): %d %s", method, path, user, response.Code, response.Body.String())
				}
			}
			count++
		}
	}
	if count != 7 {
		t.Fatalf("expected seven protected group operations, got %d", count)
	}
}

func TestGroupBodyBudgetIsBoundedBeforeJSONValidation(t *testing.T) {
	issuer := testIssuer(t)
	handler := newAuthTestRouter(t, issuer)
	token, err := issuer.Issue("01935000-0000-7000-8000-000000000001", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	limits, err := httpx.RequestBodyLimits(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 11 || limits["PATCH /me/preferences"].Bytes != 8192 || limits["POST /teacher/imports/{id}/sources"].Bytes != (25<<20)+(128<<10) || limits["POST /teacher/media"].Bytes != (50<<20)+(128<<10) || limits["POST /teacher/media/{id}/replace"].Bytes != (50<<20)+(128<<10) || limits["PUT /teacher/imports/{id}/review"].Bytes != 8<<20 || limits["POST /teacher/question-groups"].Bytes != 4<<20 || limits["PUT /teacher/question-groups/{id}"].Bytes != 4<<20 ||
		limits["POST /auth/login"].Bytes != 8<<10 || limits["POST /auth/google"].Bytes != 8<<10 || limits["POST /join/preview"].Bytes != 8<<10 || limits["POST /app/classes/join"].Bytes != 8<<10 {
		t.Fatalf("unexpected contract budgets: %v", limits)
	}
	for _, bytes := range []int{2 << 20, (4 << 20) + 1} {
		for _, chunked := range []bool{false, true} {
			request := httptest.NewRequest(http.MethodPost, "/teacher/question-groups", strings.NewReader(strings.Repeat(" ", bytes)+"{}"))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			if chunked {
				request.ContentLength = -1
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			want := http.StatusBadRequest
			if bytes > 4<<20 {
				want = http.StatusRequestEntityTooLarge
			}
			if response.Code != want {
				t.Fatalf("%d bytes chunked=%v: got %d %s", bytes, chunked, response.Code, response.Body.String())
			}
		}
	}
}
