package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/platform/httpx"
)

const (
	nulSession = "019535d9-3df7-79fb-b466-fa907fa17f9e"
	nulMarker  = "@NUL@"
)

type nulAnswer struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

type nulRequest struct {
	pattern     string
	target      string
	contentType string
	body        string
	language    string
}

func (c nulRequest) sendThrough(t *testing.T, validate func(http.Handler) http.Handler) (reached bool, rec *httptest.ResponseRecorder) {
	t.Helper()
	handler := validate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	method, _, _ := strings.Cut(c.pattern, " ")
	req := httptest.NewRequest(method, c.target, strings.NewReader(c.body))
	req.Pattern = c.pattern
	if c.contentType != "" {
		req.Header.Set("Content-Type", c.contentType)
	}
	if c.language != "" {
		req.Header.Set("Accept-Language", c.language)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return reached, rec
}

func answerOf(t *testing.T, rec *httptest.ResponseRecorder) nulAnswer {
	t.Helper()
	var answer nulAnswer
	if err := json.NewDecoder(rec.Body).Decode(&answer); err != nil {
		t.Fatalf("the response is not the error envelope: %v", err)
	}
	return answer
}

func beaconEvent(meta string) string {
	return `{"kind":"paste","occurredAt":"2026-10-11T10:00:00Z","clientSeq":1,"meta":` + meta + `}`
}

func TestANulInAJSONBodyIsRefusedWhereverItSits(t *testing.T) {
	const beacon = "POST /app/attempts/{id}/events"
	const beaconTarget = "/app/attempts/" + nulSession + "/events"
	flush := func(event string) string {
		return `{"sessionId":"` + nulSession + `","events":[` + event + `]}`
	}
	for _, tc := range []struct {
		name  string
		input nulRequest
		field string
	}{
		{
			name:  "a string at the top of the body",
			input: nulRequest{pattern: "POST /teacher/tests", target: "/teacher/tests", body: `{"title":"Đề ` + nulMarker + `"}`},
			field: "title",
		},
		{
			name: "a string in an object inside an array",
			input: nulRequest{pattern: "POST /teacher/questions", target: "/teacher/questions",
				body: `{"type":"single_choice","prompt":"Chọn","points":1,"options":[{"text":"a","isCorrect":true},{"text":"b` + nulMarker + `","isCorrect":false}]}`},
			field: "options.1.text",
		},
		{
			name: "an element of an array of strings",
			input: nulRequest{pattern: "POST /teacher/questions", target: "/teacher/questions",
				body: `{"type":"short_answer","prompt":"Viết","points":1,"tags":["a","b` + nulMarker + `"]}`},
			field: "tags.1",
		},
		{
			name:  "a string in a free-form object several levels down",
			input: nulRequest{pattern: beacon, target: beaconTarget, body: flush(beaconEvent(`{"detail":{"text":"x` + nulMarker + `"}}`))},
			field: "events.0.meta.detail.text",
		},
		{
			name:  "an array in an array in a free-form object",
			input: nulRequest{pattern: beacon, target: beaconTarget, body: flush(beaconEvent(`{"rows":[["x","y` + nulMarker + `"]]}`))},
			field: "events.0.meta.rows.0.1",
		},
		{
			name:  "an object member name, named by the object that holds it",
			input: nulRequest{pattern: beacon, target: beaconTarget, body: flush(beaconEvent(`{"k` + nulMarker + `":1}`))},
			field: "events.0.meta",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.contentType = "application/json"
			validate := validatorUnderTest(t)

			clean := tc.input
			clean.body = strings.ReplaceAll(tc.input.body, nulMarker, "x")
			if reached, rec := clean.sendThrough(t, validate); !reached {
				t.Fatalf("the same body without the NUL was refused with %d: %s", rec.Code, rec.Body.String())
			}

			refused := tc.input
			refused.body = strings.ReplaceAll(tc.input.body, nulMarker, `\u0000`)
			reached, rec := refused.sendThrough(t, validate)
			if reached {
				t.Fatal("a body holding a NUL reached the handler")
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			answer := answerOf(t, rec)
			if want := `Trường "` + tc.field + `" không hợp lệ.`; answer.Error.Code != "VALIDATION_FAILED" || answer.Error.Message != want {
				t.Errorf("answer = %s %q, want VALIDATION_FAILED %q", answer.Error.Code, answer.Error.Message, want)
			}
			if _, named := answer.Error.Details[tc.field]; !named || len(answer.Error.Details) != 1 {
				t.Errorf("details = %v, want only %q", answer.Error.Details, tc.field)
			}
		})
	}
}

func TestANulInAJSONBodyIsRefusedInEnglishWhenTheCallerPrefersIt(t *testing.T) {
	_, rec := nulRequest{
		pattern: "POST /teacher/tests", target: "/teacher/tests", contentType: "application/json", language: "en",
		body: `{"title":"a\u0000b"}`,
	}.sendThrough(t, validatorUnderTest(t))

	answer := answerOf(t, rec)
	if rec.Code != http.StatusBadRequest || answer.Error.Message != `The field "title" is not valid.` {
		t.Fatalf("answer = %d %q, want 400 and the English sentence", rec.Code, answer.Error.Message)
	}
	if got := answer.Error.Details["title"]; got != "Must not contain the NUL character (U+0000)." {
		t.Errorf("details.title = %v, want the English sentence", got)
	}
}

func TestABackslashFollowedByU0000IsTextAndNotANul(t *testing.T) {
	reached, rec := nulRequest{
		pattern: "POST /teacher/tests", target: "/teacher/tests", contentType: "application/json",
		body: `{"title":"\\u0000"}`,
	}.sendThrough(t, validatorUnderTest(t))

	if !reached {
		t.Fatalf("a title that spells out \\u0000 was refused with %d: %s", rec.Code, rec.Body.String())
	}
}

func TestANulInTheJSONTheBeaconCarriesAsTextIsRefused(t *testing.T) {
	const event = `{"kind":"pa` + nulMarker + `ste","occurredAt":"2026-10-11T10:00:00Z","clientSeq":1}`
	input := nulRequest{
		pattern: "POST /app/attempts/{id}/events", target: "/app/attempts/" + nulSession + "/events", contentType: "text/plain;charset=UTF-8",
		body: `{"beaconToken":"token","sessionId":"` + nulSession + `","events":[` + event + `]}`,
	}
	validate := validatorUnderTest(t)

	clean := input
	clean.body = strings.ReplaceAll(input.body, nulMarker, "x")
	if reached, rec := clean.sendThrough(t, validate); !reached {
		t.Fatalf("the beacon without the NUL was refused with %d: %s", rec.Code, rec.Body.String())
	}

	refused := input
	refused.body = strings.ReplaceAll(input.body, nulMarker, `\u0000`)
	reached, rec := refused.sendThrough(t, validate)
	if reached || rec.Code != http.StatusBadRequest {
		t.Fatalf("the beacon holding a NUL: reached = %v, status = %d, want refused with 400", reached, rec.Code)
	}
	if answer := answerOf(t, rec); answer.Error.Code != "VALIDATION_FAILED" || answer.Error.Message != `Trường "events.0.kind" không hợp lệ.` {
		t.Errorf("answer = %s %q", answer.Error.Code, answer.Error.Message)
	}
}

func TestATextBeaconThatIsNotJSONIsLeftToTheHandler(t *testing.T) {
	reached, rec := nulRequest{
		pattern: "POST /app/attempts/{id}/events", target: "/app/attempts/" + nulSession + "/events", contentType: "text/plain",
		body: `not json, and it spells out \u0000`,
	}.sendThrough(t, validatorUnderTest(t))

	if !reached {
		t.Fatalf("a text beacon that is not JSON was refused with %d: %s", rec.Code, rec.Body.String())
	}
}

func TestANulInAQueryParameterIsRefused(t *testing.T) {
	const pattern = "GET /teacher/tests"
	validate := validatorUnderTest(t)
	for _, tc := range []struct {
		name     string
		target   string
		language string
		message  string
		field    string
	}{
		{name: "a value", target: "/teacher/tests?q=a%00b", message: `Tham số "q" không hợp lệ.`, field: "q"},
		{name: "one of several values", target: "/teacher/tests?tag=ok&tag=%00", message: `Tham số "tag" không hợp lệ.`, field: "tag"},
		{name: "a value, in English", target: "/teacher/tests?q=%00", language: "en", message: `The parameter "q" is not valid.`, field: "q"},
		{name: "a name", target: "/teacher/tests?a%00b=1", message: "Dữ liệu gửi lên không hợp lệ."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reached, rec := nulRequest{pattern: pattern, target: tc.target, language: tc.language}.sendThrough(t, validate)
			if reached || rec.Code != http.StatusBadRequest {
				t.Fatalf("reached = %v, status = %d, want refused with 400", reached, rec.Code)
			}
			answer := answerOf(t, rec)
			if answer.Error.Code != "VALIDATION_FAILED" || answer.Error.Message != tc.message {
				t.Errorf("answer = %s %q, want VALIDATION_FAILED %q", answer.Error.Code, answer.Error.Message, tc.message)
			}
			if _, named := answer.Error.Details[tc.field]; tc.field != "" && !named {
				t.Errorf("details = %v, want %q named", answer.Error.Details, tc.field)
			}
		})
	}

	for _, target := range []string{"/teacher/tests?q=ab", "/teacher/tests?q=%2500", "/teacher/tests?q=%E1%BB%87"} {
		if reached, rec := (nulRequest{pattern: pattern, target: target}).sendThrough(t, validate); !reached {
			t.Errorf("%s was refused with %d: %s", target, rec.Code, rec.Body.String())
		}
	}
}

func TestANulInAPathSegmentTheSchemaDoesNotConstrainIsRefused(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(`
openapi: 3.1.0
info: {title: free text path, version: "1"}
paths:
  /things/{name}:
    get:
      parameters:
        - {name: name, in: path, required: true, schema: {type: string}}
      responses: {'200': {description: ok}}
`))
	if err != nil {
		t.Fatal(err)
	}
	validate, err := httpx.ValidateRequests(spec)
	if err != nil {
		t.Fatal(err)
	}

	if reached, rec := (nulRequest{pattern: "GET /things/{name}", target: "/things/ab"}).sendThrough(t, validate); !reached {
		t.Fatalf("a plain segment was refused with %d: %s", rec.Code, rec.Body.String())
	}
	reached, rec := nulRequest{pattern: "GET /things/{name}", target: "/things/a%00b"}.sendThrough(t, validate)
	if reached || rec.Code != http.StatusBadRequest {
		t.Fatalf("a segment holding a NUL: reached = %v, status = %d, want refused with 400", reached, rec.Code)
	}
	if answer := answerOf(t, rec); answer.Error.Code != "VALIDATION_FAILED" {
		t.Errorf("code = %s, want VALIDATION_FAILED", answer.Error.Code)
	}
}
