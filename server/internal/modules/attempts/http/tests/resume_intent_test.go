package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

func refusedStart(t *testing.T, body *openapi.StartOrResumeAttemptJSONRequestBody, outcome error) (*httptest.ResponseRecorder, []command.StartOrResume) {
	t.Helper()
	var received []command.StartOrResume
	student := uuid.New()
	app := &application.Application{Commands: application.Commands{StartOrResume: cqrs.HandlerFunc[command.StartOrResume, domain.Session](func(_ context.Context, cmd command.StartOrResume) (domain.Session, error) {
		received = append(received, cmd)
		return domain.Session{}, outcome
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)

	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: student.String()}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out, err := transport.StartOrResumeAttempt(r.Context(), openapi.StartOrResumeAttemptRequestObject{Id: uuid.New(), Body: body})
			if err != nil {
				t.Fatal(err)
			}
			if err := out.VisitStartOrResumeAttemptResponse(w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/app/assignments/x/attempts", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response, received
}

func refusalCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body refusal
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v in %s", err, response.Body.String())
	}
	return body.Error.Code
}

func TestAContinueOfAnEndedAttemptAnswers409AttemptClosed(t *testing.T) {
	attempt := uuid.New()
	response, received := refusedStart(t, &openapi.StartOrResumeAttemptJSONRequestBody{Resume: &attempt}, domain.ErrAttemptClosed)

	if response.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", response.Code, response.Body.String())
	}
	if code := refusalCode(t, response); code != string(openapi.ATTEMPTCLOSED) {
		t.Errorf("code %q, want ATTEMPT_CLOSED", code)
	}
	if len(received) != 1 || received[0].Resume != attempt.String() {
		t.Errorf("the command received %+v, want one naming attempt %s", received, attempt)
	}
}

func TestAStartWithoutABodyNamesNoAttempt(t *testing.T) {
	response, received := refusedStart(t, nil, domain.ErrLimitReached)

	if response.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", response.Code, response.Body.String())
	}
	if code := refusalCode(t, response); code != string(openapi.ATTEMPTLIMITREACHED) {
		t.Errorf("code %q, want ATTEMPT_LIMIT_REACHED", code)
	}
	if len(received) != 1 || received[0].Resume != "" {
		t.Errorf("the command received %+v, want one naming no attempt", received)
	}
}

func TestAStartWithAnEmptyBodyNamesNoAttempt(t *testing.T) {
	response, received := refusedStart(t, &openapi.StartOrResumeAttemptJSONRequestBody{}, domain.ErrLimitReached)

	if response.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", response.Code, response.Body.String())
	}
	if code := refusalCode(t, response); code != string(openapi.ATTEMPTLIMITREACHED) {
		t.Errorf("code %q, want ATTEMPT_LIMIT_REACHED", code)
	}
	if len(received) != 1 || received[0].Resume != "" {
		t.Errorf("the command received %+v, want one naming no attempt", received)
	}
}
