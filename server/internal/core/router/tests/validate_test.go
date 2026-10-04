package router_test

import (
	"cmp"
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
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
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

func TestOtherValidationFailuresNameTheirField(t *testing.T) {
	issuer := testIssuer(t)
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthTestRouter(t, issuer)

	for _, tc := range []struct {
		name           string
		path           string
		body           string
		acceptLanguage string
		want           string
	}{
		{name: "another operation's password", path: "/auth/login", body: `{"email":"a@b.com","password":"short"}`, acceptLanguage: "en", want: `The field "password" is not valid.`},
		{name: "newPassword absent", path: "/auth/change-password", body: `{"currentPassword":"matkhau1"}`, acceptLanguage: "en", want: `The field "newPassword" is not valid.`},
		{name: "another field beside a good newPassword", path: "/auth/change-password", body: `{"currentPassword":5,"newPassword":"matkhau1"}`, acceptLanguage: "en", want: `The field "currentPassword" is not valid.`},
		{name: "a field the validator names", path: "/teacher/students", body: `{"email":"a@b.com","fullName":""}`, acceptLanguage: "en", want: `The field "fullName" is not valid.`},
		{name: "another operation's password, no language asked", path: "/auth/login", body: `{"email":"a@b.com","password":"short"}`, want: `Trường "password" không hợp lệ.`},
		{name: "newPassword absent, no language asked", path: "/auth/change-password", body: `{"currentPassword":"matkhau1"}`, want: `Trường "newPassword" không hợp lệ.`},
		{name: "another field beside a good newPassword, no language asked", path: "/auth/change-password", body: `{"currentPassword":5,"newPassword":"matkhau1"}`, want: `Trường "currentPassword" không hợp lệ.`},
		{name: "a field the validator names, no language asked", path: "/teacher/students", body: `{"email":"a@b.com","fullName":""}`, want: `Trường "fullName" không hợp lệ.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSONAs(t, handler, tc.path, tc.body, token, tc.acceptLanguage)

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

const genericValidationSentence = "Dữ liệu gửi lên không hợp lệ."

func sendIn(t *testing.T, handler http.Handler, method, path, token, acceptLanguage, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func wantValidationFailure(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	code, message := errorCodeAndMessage(t, rec)
	if code != "VALIDATION_FAILED" {
		t.Errorf("error code = %q, want VALIDATION_FAILED", code)
	}
	if message != want {
		t.Errorf("message = %q, want %q", message, want)
	}
}

var unnamed = map[string]string{}

func wrongTypedFirstProperty(t *testing.T, op *openapi3.Operation) (property, body string, ok bool) {
	t.Helper()
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return "", "", false
	}
	media := op.RequestBody.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil || media.Schema.Value == nil || len(media.Schema.Value.Properties) == 0 {
		return "", "", false
	}
	names := make([]string, 0, len(media.Schema.Value.Properties))
	for name := range media.Schema.Value.Properties {
		names = append(names, name)
	}
	property = slices.Min(names)
	var wrong any = "x"
	if schema := media.Schema.Value.Properties[property].Value; schema != nil && schema.Type != nil && schema.Type.Includes("string") {
		wrong = 5
	}
	encoded, err := json.Marshal(map[string]any{property: wrong})
	if err != nil {
		t.Fatal(err)
	}
	return property, string(encoded), true
}

func pathWithParameters(path string, item *openapi3.PathItem, op *openapi3.Operation) string {
	for _, parameters := range []openapi3.Parameters{item.Parameters, op.Parameters} {
		for _, parameter := range parameters {
			if parameter.Value == nil || parameter.Value.In != "path" {
				continue
			}
			value := "x"
			if schema := parameter.Value.Schema; schema != nil && schema.Value != nil {
				switch {
				case schema.Value.Format == "uuid":
					value = "019535d9-3df7-79fb-b466-fa907fa17f9e"
				case schema.Value.Type != nil && schema.Value.Type.Includes("integer"):
					value = "1"
				}
			}
			path = strings.ReplaceAll(path, "{"+parameter.Value.Name+"}", value)
		}
	}
	return path
}

func TestEveryJSONBodyOperationNamesAFieldWithTheWrongType(t *testing.T) {
	spec, err := openapi.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	requirements, err := httpx.PermissionRequirements(spec, "bearerAuth")
	if err != nil {
		t.Fatal(err)
	}
	open := httpx.OpenRoutes(spec, "bearerAuth")
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	checked := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			property, body, ok := wrongTypedFirstProperty(t, op)
			if !ok {
				continue
			}
			checked[op.OperationID] = true
			pattern := method + " " + path
			user := adminUser
			if _, isOpen := open[pattern]; isOpen {
				user = ""
			} else if slices.Contains(requirements[pattern].Keys(), access.LearningTakeTests) {
				user = studentUser
			}
			t.Run(op.OperationID, func(t *testing.T) {
				rec := sendAs(t, handler, issuer, method, pathWithParameters(path, item, op), user, body)

				want := `Trường "` + property + `" không hợp lệ.`
				answer := rec.Body.String()
				var code, message string
				if rec.Code == http.StatusBadRequest {
					code, message = errorCodeAndMessage(t, rec)
				}
				named := code == "VALIDATION_FAILED" && message == want
				reason, excused := unnamed[op.OperationID]
				switch {
				case excused && named:
					t.Errorf("%s names its field now; take it out of unnamed (%s)", pattern, reason)
				case !excused && !named:
					t.Errorf("%s with %s: status = %d, answer = %s, want 400 VALIDATION_FAILED %q", pattern, body, rec.Code, answer, want)
				}
			})
		}
	}

	for operation := range unnamed {
		if !checked[operation] {
			t.Errorf("unnamed lists %s, which is not an operation with a JSON body in the contract", operation)
		}
	}
	if len(checked) < 40 {
		t.Errorf("only %d operations were checked; the walk is not reading what it thinks it is", len(checked))
	}
}

func TestTwoFailingFieldsAlwaysNameTheSame(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	for attempt := range 50 {
		rec := sendAs(t, handler, issuer, http.MethodPost, "/teacher/classes", adminUser, `{"name": 5, "selfJoinEnabled": 5}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status = %d, want 400", attempt, rec.Code)
		}
		if _, message := errorCodeAndMessage(t, rec); message != `Trường "name" không hợp lệ.` {
			t.Fatalf("attempt %d: message = %q, want the field \"name\"", attempt, message)
		}
	}
}

func TestAMissingRequiredPropertyIsNamedOnBothValidatorPaths(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	admin, err := issuer.Issue(adminUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name           string
		path           string
		token          string
		body           string
		acceptLanguage string
		want           string
	}{
		{name: "one of two absent", path: "/auth/login", body: `{"password":"long-enough"}`, want: `Trường "email" không hợp lệ.`},
		{name: "both absent names the contract's first", path: "/auth/login", body: `{}`, want: `Trường "email" không hợp lệ.`},
		{name: "a value that breaks a rule comes first", path: "/auth/login", body: `{"password": 5}`, want: `Trường "password" không hợp lệ.`},
		{name: "the built-in validator agrees", path: "/teacher/students", token: admin, body: `{"email":"a@b.com"}`, want: `Trường "fullName" không hợp lệ.`},
		{name: "an unknown property", path: "/auth/login", body: `{"email":"a@b.com","password":"long-enough","admin":true}`, want: genericValidationSentence},
		{name: "an unknown property on the built-in validator", path: "/teacher/students", token: admin, body: `{"email":"a@b.com","fullName":"An","admin":true}`, want: genericValidationSentence},
		{name: "a null body", path: "/auth/login", body: `null`, want: genericValidationSentence},
		{name: "an array body", path: "/auth/login", body: `[]`, want: genericValidationSentence},
		{name: "English preferred", path: "/auth/login", body: `{"password":"long-enough"}`, acceptLanguage: "en", want: `The field "email" is not valid.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantValidationFailure(t, sendIn(t, handler, http.MethodPost, tc.path, tc.token, tc.acceptLanguage, tc.body), tc.want)
		})
	}
}

func TestAMalformedAnswerKeyIsNamedInEnglishWhenAsked(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	student, err := issuer.Issue(studentUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}

	rec := sendIn(t, handler, http.MethodPatch, saveAnswersPath, student, "en", saveAnswersKeyedBy(t, "not-a-uuid"))
	wantValidationFailure(t, rec, `The field "answers" is not valid.`)
}

const (
	classMembersPath = "/teacher/classes/019535d9-3df7-79fb-b466-fa907fa17f9e/members"
	groupCopyPath    = "/teacher/question-groups/019535d9-3df7-79fb-b466-fa907fa17f9e/copy"
)

func TestAMalformedEmailInABodyNamesItsField(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	admin, err := issuer.Issue(adminUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name           string
		path           string
		token          string
		body           string
		acceptLanguage string
		want           string
	}{
		{name: "sign-in", path: "/auth/login", body: `{"email":"not-an-email","password":"long-enough"}`, want: `Trường "email" không hợp lệ.`},
		{name: "sign-in, English preferred", path: "/auth/login", body: `{"email":"not-an-email","password":"long-enough"}`, acceptLanguage: "en", want: `The field "email" is not valid.`},
		{name: "a new student", path: "/teacher/students", token: admin, body: `{"email":"not-an-email","fullName":"An"}`, want: `Trường "email" không hợp lệ.`},
		{name: "a new student, English preferred", path: "/teacher/students", token: admin, body: `{"email":"not-an-email","fullName":"An"}`, acceptLanguage: "en", want: `The field "email" is not valid.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantValidationFailure(t, sendIn(t, handler, http.MethodPost, tc.path, tc.token, tc.acceptLanguage, tc.body), tc.want)
		})
	}

	for name, body := range map[string]string{
		"a plain address":      `{"email":"a@b.com","password":"long-enough"}`,
		"an address in angles": `{"email":"An <a@b.com>","password":"long-enough"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if rec := sendIn(t, handler, http.MethodPost, "/auth/login", "", "", body); rec.Code == http.StatusBadRequest {
				t.Fatalf("an address the decoder accepts was refused: %s", rec.Body.String())
			}
		})
	}
}

func TestADecodeFailureNeverEchoesTheDecoder(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	admin, err := issuer.Issue(adminUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	student, err := issuer.Issue(studentUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		method string
		path   string
		token  string
		body   string
	}{
		{name: "a uuid the built-in validator let through", method: http.MethodPost, path: classMembersPath, token: admin, body: `{"userId":"nope"}`},
		{name: "a uuid the 2020-12 validator let through", method: http.MethodPost, path: groupCopyPath, token: admin, body: `{"expectedRevision":1,"ownerSectionId":"nope"}`},
		{name: "a session id", method: http.MethodPatch, path: saveAnswersPath, token: student, body: `{"sessionId":"nope"}`},
		{name: "a day the month does not have", method: http.MethodPost, path: "/teacher/assignments/019535d9-3df7-79fb-b466-fa907fa17f9e/reopen", token: admin, body: `{"closesAt":"2026-02-30T00:00:00Z","reason":"Mở lại cho lớp"}`},
	} {
		for _, language := range []struct {
			name           string
			acceptLanguage string
			want           string
		}{
			{name: "no language asked", want: genericValidationSentence},
			{name: "English preferred", acceptLanguage: "en", want: "The submitted data is not valid."},
		} {
			t.Run(tc.name+", "+language.name, func(t *testing.T) {
				rec := sendIn(t, handler, tc.method, tc.path, tc.token, language.acceptLanguage, tc.body)

				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
				}
				code, message := errorCodeAndMessage(t, rec)
				if code != "VALIDATION_FAILED" {
					t.Errorf("error code = %q, want VALIDATION_FAILED", code)
				}
				if message != language.want {
					t.Errorf("message = %q, want %q", message, language.want)
				}
				for _, leak := range []string{"decode", "unmarshal", "invalid", "parsing"} {
					if strings.Contains(strings.ToLower(message), leak) {
						t.Errorf("message %q carries the decoder's %q", message, leak)
					}
				}
			})
		}
	}
}

func TestTheValidatorRefusesNoIdTheDecoderAccepts(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	for name, userID := range map[string]string{
		"hyphenated": "019535d9-3df7-79fb-b466-fa907fa17f9f",
		"braced":     "{019535d9-3df7-79fb-b466-fa907fa17f9f}",
		"urn":        "urn:uuid:019535d9-3df7-79fb-b466-fa907fa17f9f",
		"no hyphens": "019535d93df779fbb466fa907fa17f9f",
	} {
		t.Run(name, func(t *testing.T) {
			rec := sendAs(t, handler, issuer, http.MethodPost, classMembersPath, adminUser, `{"userId":"`+userID+`"}`)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501 from a router with no classes module: %s", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("an option id inside an answer, which the handler reads", func(t *testing.T) {
		body := `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{"019535d9-3df7-79fb-b466-fa907fa17fa0":{"type":"choice","optionIds":["not-a-uuid"]}}}`
		rec := sendAs(t, handler, issuer, http.MethodPatch, saveAnswersPath, studentUser, body)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want 501 from a router with no attempts module: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAMalformedParameterNamesTheParameter(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	admin, err := issuer.Issue(adminUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name           string
		path           string
		token          string
		acceptLanguage string
		want           string
	}{
		{name: "a path parameter", path: "/teacher/classes/not-a-uuid", token: admin, want: `Tham số "id" không hợp lệ.`},
		{name: "a path parameter, English preferred", path: "/teacher/classes/not-a-uuid", token: admin, acceptLanguage: "en", want: `The parameter "id" is not valid.`},
		{name: "a path parameter from nobody", path: "/teacher/classes/not-a-uuid", want: `Tham số "id" không hợp lệ.`},
		{name: "a path parameter from nobody, English preferred", path: "/teacher/classes/not-a-uuid", acceptLanguage: "en", want: `The parameter "id" is not valid.`},
		{name: "a query parameter that does not bind", path: "/teacher/classes?limit=abc", token: admin, want: `Tham số "limit" không hợp lệ.`},
		{name: "a query parameter the validator refuses", path: "/teacher/classes?limit=100000", token: admin, want: `Tham số "limit" không hợp lệ.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantValidationFailure(t, sendIn(t, handler, http.MethodGet, tc.path, tc.token, tc.acceptLanguage, ""), tc.want)
		})
	}
}

const (
	repeatedAnswersBody = `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{"019535d9-3df7-79fb-b466-fa907fa17fa0":5},"answers":{}}`
	textBeaconPath      = "/app/attempts/019535d9-3df7-79fb-b466-fa907fa17f9e/events"
)

func saveAnswersWithEvent(event string) string {
	return `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","events":[` + event + `]}`
}

func TestABodyThatRepeatsAMemberNameIsRefused(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	admin, err := issuer.Issue(adminUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	student, err := issuer.Issue(studentUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name           string
		method         string
		path           string
		token          string
		contentType    string
		acceptLanguage string
		body           string
		want           string
	}{
		{name: "at the top level, on the 2020-12 validator", method: http.MethodPost, path: "/teacher/classes", token: admin, body: `{"name":"Lớp A","name":"Lớp B"}`, want: `Trường "name" không hợp lệ.`},
		{name: "at the top level, on the built-in validator", method: http.MethodPost, path: "/teacher/students", token: admin, body: `{"email":"a@b.com","fullName":"An","fullName":"Bình"}`, want: `Trường "fullName" không hợp lệ.`},
		{name: "a name written with an escape", method: http.MethodPost, path: "/teacher/classes", token: admin, body: `{"name":"Lớp A","n\u0061me":"Lớp B"}`, want: `Trường "name" không hợp lệ.`},
		{name: "nested in an object", method: http.MethodPatch, path: saveAnswersPath, token: student, body: `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{"019535d9-3df7-79fb-b466-fa907fa17fa0":{"type":"choice","optionIds":[],"optionIds":[]}}}`, want: `Trường "answers" không hợp lệ.`},
		{name: "nested in an array", method: http.MethodPatch, path: saveAnswersPath, token: student, body: saveAnswersWithEvent(`{"kind":"paste","kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0}`), want: `Trường "events" không hợp lệ.`},
		{name: "an earlier occurrence the contract refuses", method: http.MethodPatch, path: saveAnswersPath, token: student, body: repeatedAnswersBody, want: `Trường "answers" không hợp lệ.`},
		{name: "under a Content-Type whose parameters repeat", method: http.MethodPatch, path: saveAnswersPath, token: student, contentType: "application/json; charset=a; charset=b", body: repeatedAnswersBody, want: `Trường "answers" không hợp lệ.`},
		{name: "English preferred", method: http.MethodPost, path: "/teacher/classes", token: admin, acceptLanguage: "en", body: `{"name":"Lớp A","name":"Lớp B"}`, want: `The field "name" is not valid.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", cmp.Or(tc.contentType, "application/json"))
			req.Header.Set("Authorization", "Bearer "+tc.token)
			if tc.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tc.acceptLanguage)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			wantValidationFailure(t, rec, tc.want)
		})
	}
}

func TestANestedRepeatOnAFlatOperationIsRefusedByTheSchema(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	rec := sendAs(t, handler, issuer, http.MethodPost, "/teacher/classes", adminUser, `{"name":"Lớp A","description":{"a":1,"a":2}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if code, _ := errorCodeAndMessage(t, rec); code != "VALIDATION_FAILED" {
		t.Errorf("error code = %q, want VALIDATION_FAILED", code)
	}
}

func TestAPropertyThatDiffersOnlyInCaseIsRefusedAsUnknown(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	rec := sendAs(t, handler, issuer, http.MethodPost, "/teacher/classes", adminUser, `{"name":"Lớp A","Name":"Lớp B"}`)

	wantValidationFailure(t, rec, genericValidationSentence)
}

func TestABodyThatRepeatsNoMemberReachesTheHandler(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	for _, tc := range []struct {
		name   string
		method string
		path   string
		user   string
		body   string
	}{
		{name: "the same names in sibling objects", method: http.MethodPatch, path: saveAnswersPath, user: studentUser, body: `{"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","answers":{"019535d9-3df7-79fb-b466-fa907fa17fa0":{"type":"choice","optionIds":[]},"019535d9-3df7-79fb-b466-fa907fa17fa1":{"type":"choice","optionIds":[]}},"events":[{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0},{"kind":"paste","occurredAt":"2026-10-04T08:00:01Z","clientSeq":1}]}`},
		{name: "a nested name that is also a top-level one", method: http.MethodPatch, path: saveAnswersPath, user: studentUser, body: saveAnswersWithEvent(`{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0,"meta":{"sessionId":"x"}}`)},
		{name: "a string twice after an object in an array", method: http.MethodPatch, path: saveAnswersPath, user: studentUser, body: saveAnswersWithEvent(`{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0,"meta":{"a":[{"b":1},"s","s"]}}`)},
		{name: "a string after an empty object in an array", method: http.MethodPatch, path: saveAnswersPath, user: studentUser, body: saveAnswersWithEvent(`{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0,"meta":{"a":[{},"s"]}}`)},
		{name: "names that differ only in case in a free-form object", method: http.MethodPatch, path: saveAnswersPath, user: studentUser, body: saveAnswersWithEvent(`{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0,"meta":{"a":1,"A":2}}`)},
		{name: "a null member", method: http.MethodPost, path: "/teacher/classes", user: adminUser, body: `{"name":"Lớp A","description":null}`},
		{name: "one value", method: http.MethodPost, path: "/teacher/classes", user: adminUser, body: `{"name":"Lớp A"}`},
		{name: "bytes after the first value", method: http.MethodPost, path: "/teacher/classes", user: adminUser, body: `{"name":"Lớp A"} x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := sendAs(t, handler, issuer, tc.method, tc.path, tc.user, tc.body)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want 501 from a router with no modules: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTheTextBeaconIsNotReadForRepeats(t *testing.T) {
	handler := newAuthTestRouter(t, testIssuer(t))
	send := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, textBeaconPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	const rest = `"sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9f","events":[{"kind":"paste","occurredAt":"2026-10-04T08:00:00Z","clientSeq":0}]}`

	once := send(`{"beaconToken":"t",` + rest)
	twice := send(`{"beaconToken":"t","beaconToken":"t",` + rest)

	if once == http.StatusBadRequest {
		t.Fatalf("status = %d, want the text beacon to reach its handler", once)
	}
	if twice != once {
		t.Fatalf("status with beaconToken twice = %d, want %d as with it once", twice, once)
	}
}
