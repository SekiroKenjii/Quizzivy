package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/gen/openapi"
)

// api/openapi.yaml is the source of truth, but oapi-codegen only generates
// types and binds JSON from it -- it enforces none of the constraints. Before
// the validator, `password` with `minLength: 8` accepted an empty string and
// `format: email` accepted anything, so every handler had to restate its own
// rules in Go or silently have none.

func postJSON(t *testing.T, router http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response is not the error envelope: %v", err)
	}
	code, _ := body["error"]["code"].(string)
	return code
}

func TestTheContractsConstraintsAreEnforced(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))

	for name, body := range map[string]string{
		"password below minLength": `{"email":"a@b.com","password":"short"}`,
		"password absent":          `{"email":"a@b.com"}`,
		"email absent":             `{"password":"long-enough"}`,
		"email not an email":       `{"email":"not-an-email","password":"long-enough"}`,
		"unknown field":            `{"email":"a@b.com","password":"long-enough","admin":true}`,
		"wrong type":               `{"email":"a@b.com","password":12345678}`,
		"not json at all":          `pretzel`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := postJSON(t, router, "/auth/login", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if got := errorCode(t, rec); got != "VALIDATION_FAILED" {
				t.Errorf("error code = %q, want VALIDATION_FAILED", got)
			}
		})
	}
}

func TestAWellFormedRequestReachesTheHandler(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))
	rec := postJSON(t, router, "/auth/login", `{"email":"a@b.com","password":"long-enough"}`)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("a valid body was rejected: %s", rec.Body.String())
	}
}

func TestValidationMessagesDoNotEchoTheSchema(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))
	rec := postJSON(t, router, "/auth/login", `{"email":"a@b.com","password":"short"}`)

	body := rec.Body.String()
	for _, leak := range []string{"minLength", "properties", "schema", "openapi"} {
		if strings.Contains(strings.ToLower(body), leak) {
			t.Errorf("validation error leaks %q: %s", leak, body)
		}
	}
}

func TestAuthenticationIsDecidedBeforeValidation(t *testing.T) {
	router := newAuthTestRouter(t, testIssuer(t))
	rec := postJSON(t, router, "/auth/change-password", `{"nonsense":true}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 -- validation ran before authentication", rec.Code)
	}
}

func TestMiddlewareRunsInTheOrderItIsWritten(t *testing.T) {
	var order []string
	mark := func(name string) openapi.MiddlewareFunc {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	mux := http.NewServeMux()
	strict := openapi.NewStrictHandler(&Server{}, nil)
	handler := openapi.HandlerWithOptions(strict, openapi.StdHTTPServerOptions{
		BaseRouter:  mux,
		Middlewares: inExecutionOrder(mark("first"), mark("second"), mark("third")),
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if got := strings.Join(order, ","); got != "first,second,third" {
		t.Fatalf("execution order = %s, want first,second,third", got)
	}
}

func TestPathParametersAreValidatedToo(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/classes/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	newAuthTestRouter(t, issuer).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a malformed uuid path parameter", rec.Code)
	}
}
