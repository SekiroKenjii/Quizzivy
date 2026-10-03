package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/domain"
)

// api/openapi.yaml is the source of truth, but oapi-codegen only generates
// types and binds JSON from it -- it enforces none of the constraints. Before
// the validator, `password` with `minLength: 8` accepted an empty string and
// `format: email` accepted anything, so every handler had to restate its own
// rules in Go or silently have none.

func postJSON(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
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
	handler := newAuthTestRouter(t, testIssuer(t))

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
			rec := postJSON(t, handler, "/auth/login", body)
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
	handler := newAuthTestRouter(t, testIssuer(t))
	rec := postJSON(t, handler, "/auth/login", `{"email":"a@b.com","password":"long-enough"}`)
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("a valid body was rejected: %s", rec.Body.String())
	}
}

func TestValidationMessagesDoNotEchoTheSchema(t *testing.T) {
	handler := newAuthTestRouter(t, testIssuer(t))
	rec := postJSON(t, handler, "/auth/login", `{"email":"a@b.com","password":"short"}`)

	body := rec.Body.String()
	for _, leak := range []string{"minLength", "properties", "schema", "openapi"} {
		if strings.Contains(strings.ToLower(body), leak) {
			t.Errorf("validation error leaks %q: %s", leak, body)
		}
	}
}

func TestAuthenticationIsDecidedBeforeValidation(t *testing.T) {
	handler := newAuthTestRouter(t, testIssuer(t))
	rec := postJSON(t, handler, "/auth/change-password", `{"nonsense":true}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 -- validation ran before authentication", rec.Code)
	}
}

func TestPathParametersAreValidatedToo(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/teacher/classes/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	newAuthTestRouter(t, issuer).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a malformed uuid path parameter", rec.Code)
	}
}

func TestANewPasswordNeedsADigitPunctuationOrSymbol(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000b2", "student", 0)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthTestRouter(t, issuer)

	for password, allowed := range map[string]bool{
		"matkhau1":   true,
		"mật khẩu!":  true,
		"mậtkhẩuđẹp": false,
	} {
		t.Run(password, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"newPassword": password})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/auth/change-password", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			refused := rec.Code == http.StatusBadRequest && errorCode(t, rec) == "VALIDATION_FAILED"
			if refused == allowed {
				t.Errorf("status = %d, refused = %t, want refused = %t", rec.Code, refused, !allowed)
			}
		})
	}
}

func TestATemporaryPasswordMeetsTheNewPasswordRules(t *testing.T) {
	spec, err := openapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	body := spec.Paths.Find("/auth/change-password").Post.RequestBody.Value
	rule := body.Content.Get("application/json").Schema.Value.Properties["newPassword"].Value
	pattern, err := regexp.Compile(rule.Pattern)
	if err != nil {
		t.Fatalf("newPassword pattern %q: %v", rule.Pattern, err)
	}

	for range 200 {
		password, err := domain.Passwords.Temporary()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(password) || uint64(utf8.RuneCountInString(password)) < rule.MinLength {
			t.Fatalf("temporary password %q breaks the rules a new password must meet", password)
		}
	}
}

func postJSONAs(t *testing.T, handler http.Handler, path, body, token, acceptLanguage string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func errorCodeAndMessage(t *testing.T, rec *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response is not the error envelope: %v", err)
	}
	return body.Error.Code, body.Error.Message
}

func TestABrokenPasswordRuleIsStatedInTheCallersLanguage(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000b2", "student", 0)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthTestRouter(t, issuer)

	const (
		vietnamese = "Mật khẩu mới cần từ 8 đến 512 ký tự và có số hoặc ký hiệu."
		english    = "The new password needs 8 to 512 characters and a number or symbol."
	)
	tooLong, err := json.Marshal(map[string]string{"newPassword": strings.Repeat("a", 512) + "1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		body           string
		acceptLanguage string
		want           string
	}{
		{name: "no digit or symbol", body: `{"newPassword":"mậtkhẩuđẹp"}`, want: vietnamese},
		{name: "seven characters", body: `{"newPassword":"short1!"}`, want: vietnamese},
		{name: "short and no digit or symbol", body: `{"newPassword":"short"}`, want: vietnamese},
		{name: "513 characters", body: string(tooLong), want: vietnamese},
		{name: "beside another failing field", body: `{"currentPassword":5,"newPassword":"short"}`, want: vietnamese},
		{name: "English preferred", body: `{"newPassword":"mậtkhẩuđẹp"}`, acceptLanguage: "en-US,en;q=0.9,vi;q=0.8", want: english},
		{name: "neither language preferred", body: `{"newPassword":"mậtkhẩuđẹp"}`, acceptLanguage: "fr", want: vietnamese},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSONAs(t, handler, "/auth/change-password", tc.body, token, tc.acceptLanguage)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			code, message := errorCodeAndMessage(t, rec)
			if code != "VALIDATION_FAILED" {
				t.Errorf("error code = %q, want VALIDATION_FAILED", code)
			}
			if message != tc.want {
				t.Errorf("message = %q, want %q", message, tc.want)
			}
		})
	}
}

func TestOtherValidationFailuresKeepTheirSentence(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthTestRouter(t, issuer)

	const generic = "Dữ liệu gửi lên không hợp lệ."
	for _, tc := range []struct {
		name string
		path string
		body string
		want string
	}{
		{name: "another operation's password", path: "/auth/login", body: `{"email":"a@b.com","password":"short"}`, want: generic},
		{name: "newPassword absent", path: "/auth/change-password", body: `{"currentPassword":"matkhau1"}`, want: generic},
		{name: "another field beside a good newPassword", path: "/auth/change-password", body: `{"currentPassword":5,"newPassword":"matkhau1"}`, want: generic},
		{name: "a field the validator names", path: "/teacher/students", body: `{"email":"a@b.com","fullName":""}`, want: `Trường "fullName" không hợp lệ.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSONAs(t, handler, tc.path, tc.body, token, "en")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			code, message := errorCodeAndMessage(t, rec)
			if code != "VALIDATION_FAILED" {
				t.Errorf("error code = %q, want VALIDATION_FAILED", code)
			}
			if message != tc.want {
				t.Errorf("message = %q, want %q", message, tc.want)
			}
		})
	}
}

const saveAnswersPath = "/app/attempts/019535d9-3df7-79fb-b466-fa907fa17f9e/answers"

func saveAnswersKeyedBy(t *testing.T, key string) string {
	t.Helper()
	quoted, err := json.Marshal(key)
	if err != nil {
		t.Fatal(err)
	}
	return `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{` + string(quoted) + `:{"type":"choice","optionIds":[]}}}`
}

func TestAnAnswerKeyThatIsNotAUuidIsRefusedBeforeTheHandler(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	for name, body := range map[string]string{
		"not a uuid":             saveAnswersKeyedBy(t, "not-a-uuid"),
		"braced":                 saveAnswersKeyedBy(t, "{019535d9-3df7-79fb-b466-fa907fa17f9e}"),
		"urn":                    saveAnswersKeyedBy(t, "urn:uuid:019535d9-3df7-79fb-b466-fa907fa17f9e"),
		"no hyphens":             saveAnswersKeyedBy(t, "019535d93df779fbb466fa907fa17f9e"),
		"empty":                  saveAnswersKeyedBy(t, ""),
		"36 characters, not hex": saveAnswersKeyedBy(t, "019535d9-3df7-79fb-b466-fa907fa17f9g"),
		"beside a uuid":          `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{"019535d9-3df7-79fb-b466-fa907fa17fa0":{"type":"choice","optionIds":[]},"not-a-uuid":{"type":"choice","optionIds":[]}}}`,
		"in a repeated member":   `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{"not-a-uuid":{"type":"choice","optionIds":[]}},"answers":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := sendAs(t, handler, issuer, http.MethodPatch, saveAnswersPath, studentUser, body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			code, message := errorCodeAndMessage(t, rec)
			if code != "VALIDATION_FAILED" {
				t.Errorf("error code = %q, want VALIDATION_FAILED", code)
			}
			if want := `Trường "answers" không hợp lệ.`; message != want {
				t.Errorf("message = %q, want %q", message, want)
			}
		})
	}
}

func TestAWellFormedAnswerKeyReachesTheHandler(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	for name, body := range map[string]string{
		"a hyphenated uuid": saveAnswersKeyedBy(t, "019535d9-3df7-79fb-b466-fa907fa17fa0"),
		"no answers":        `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f"}`,
		"an empty map":      `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := sendAs(t, handler, issuer, http.MethodPatch, saveAnswersPath, studentUser, body)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501 from a router with no attempts module: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOnlySaveAnswersIsKeyedByUuidToday(t *testing.T) {
	spec, err := openapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}

	var keyed []string
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			for mediaType, media := range op.RequestBody.Value.Content {
				found := map[string]bool{}
				uuidPointers(media.Schema, "", map[*openapi3.Schema]bool{}, found)
				for pointer := range found {
					if strings.HasSuffix(pointer, "/+") {
						keyed = append(keyed, method+" "+path+" "+mediaType+" "+pointer)
					}
				}
			}
		}
	}
	slices.Sort(keyed)
	if want := []string{"PATCH /app/attempts/{id}/answers application/json /answers/+"}; !slices.Equal(keyed, want) {
		t.Fatalf("body maps keyed by uuid = %v, want %v", keyed, want)
	}
}

func TestThePasswordRuleSentenceMatchesTheContract(t *testing.T) {
	spec, err := openapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}

	var takers []string
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			media := op.RequestBody.Value.Content.Get("application/json")
			if media == nil || media.Schema == nil || media.Schema.Value == nil {
				continue
			}
			if _, ok := media.Schema.Value.Properties["newPassword"]; ok {
				takers = append(takers, method+" "+path)
			}
		}
	}
	slices.Sort(takers)
	if !slices.Equal(takers, []string{"POST /auth/change-password"}) {
		t.Fatalf("operations taking newPassword = %v, want only POST /auth/change-password", takers)
	}

	body := spec.Paths.Find("/auth/change-password").Post.RequestBody.Value
	rule := body.Content.Get("application/json").Schema.Value.Properties["newPassword"].Value
	var maxLength uint64
	if rule.MaxLength != nil {
		maxLength = *rule.MaxLength
	}
	if rule.MinLength != 8 || maxLength != 512 || rule.Pattern != `[\p{N}\p{P}\p{S}]` {
		t.Errorf("newPassword rule = %d to %d characters matching %q, want 8 to 512 matching %q, which is what the sentence states",
			rule.MinLength, maxLength, rule.Pattern, `[\p{N}\p{P}\p{S}]`)
	}
}
