package httpx_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/iotest"

	gen "quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpx"
)

type endingBody struct {
	left int
	err  error
}

func (b *endingBody) Read(p []byte) (int, error) {
	n := min(len(p), b.left)
	b.left -= n
	if b.left == 0 {
		return n, b.err
	}
	return n, nil
}

type refusal struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func throughTheLimit(r *http.Request, next http.Handler) *httptest.ResponseRecorder {
	r.Pattern = "POST /auth/login"
	rec := httptest.NewRecorder()
	httpx.LimitRequestBody(nil, bodyLimit, nil)(next).ServeHTTP(rec, r)
	return rec
}

func refusalOf(t *testing.T, rec *httptest.ResponseRecorder) refusal {
	t.Helper()
	var answer refusal
	if err := json.NewDecoder(rec.Body).Decode(&answer); err != nil {
		t.Fatalf("response is not the error envelope: %v", err)
	}
	return answer
}

func wantIncomplete(t *testing.T, body io.Reader) {
	t.Helper()
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		letThrough().ServeHTTP(w, r)
	})
	rec := throughTheLimit(httptest.NewRequest(http.MethodPost, "/auth/login", body), next)

	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want 408: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := refusalOf(t, rec).Error.Code; got != "REQUEST_INCOMPLETE" {
		t.Errorf("error.code = %q, want REQUEST_INCOMPLETE", got)
	}
	if reached {
		t.Error("the handler behind the limit ran although the body was not read")
	}
}

func TestABodyThatCannotBeReadIsNotAValidationFailure(t *testing.T) {
	wantIncomplete(t, failingBody{})
}

func TestABodyCutShortIsTheSameAnswer(t *testing.T) {
	wantIncomplete(t, io.MultiReader(strings.NewReader(`{"s`), iotest.ErrReader(io.ErrUnexpectedEOF)))
}

func TestATimedOutBodyIsTheSameAnswer(t *testing.T) {
	wantIncomplete(t, iotest.ErrReader(os.ErrDeadlineExceeded))
}

func TestABodyOverTheLimitIsStill413(t *testing.T) {
	for _, c := range []struct {
		name string
		sent func() *http.Request
	}{
		{
			name: "a declared length above the limit",
			sent: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
				r.ContentLength = bodyLimit + 1
				return r
			},
		},
		{
			name: "a read that fails as it passes the limit",
			sent: func() *http.Request {
				body := &endingBody{left: bodyLimit + 1, err: errors.New("connection reset")}
				r := httptest.NewRequest(http.MethodPost, "/auth/login", body)
				r.ContentLength = -1
				return r
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := throughTheLimit(c.sent(), letThrough())

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413: %s", rec.Code, rec.Body.String())
			}
			answer := refusalOf(t, rec)
			if answer.Error.Code != "VALIDATION_FAILED" {
				t.Errorf("error.code = %q, want VALIDATION_FAILED", answer.Error.Code)
			}
			if want := "Dữ liệu gửi lên vượt quá giới hạn 1.5 MiB."; answer.Error.Message != want {
				t.Errorf("error.message = %q, want %q", answer.Error.Message, want)
			}
		})
	}
}

func TestABodyThatIsReadReachesTheHandlerWhole(t *testing.T) {
	const sent = `{"sessionId":"s-1","answers":{"q-1":{"text":"Hà Nội"}}}`
	var got []byte
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if got, err = io.ReadAll(r.Body); err != nil {
			t.Errorf("reading the body behind the limit: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	rec := throughTheLimit(httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(sent)), next)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want the handler's 204: %s", rec.Code, rec.Body.String())
	}
	if string(got) != sent {
		t.Errorf("the handler read %q, want %q", got, sent)
	}
}

func TestRequestIncompleteIsInTheContract(t *testing.T) {
	if !gen.REQUESTINCOMPLETE.Valid() {
		t.Error("REQUEST_INCOMPLETE is not a member of the contract's ErrorCode")
	}
	if string(gen.REQUESTINCOMPLETE) != string(httpx.CodeRequestIncomplete) {
		t.Errorf("httpx.CodeRequestIncomplete = %q, want the contract's %q", httpx.CodeRequestIncomplete, gen.REQUESTINCOMPLETE)
	}
}
