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
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

func refusedStartIn(t *testing.T, acceptLanguage string, outcome error) *httptest.ResponseRecorder {
	t.Helper()
	student := uuid.New()
	app := &application.Application{Commands: application.Commands{StartOrResume: cqrs.HandlerFunc[command.StartOrResume, domain.Session](func(context.Context, command.StartOrResume) (domain.Session, error) {
		return domain.Session{}, outcome
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)

	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: student.String()}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out, err := transport.StartOrResumeAttempt(r.Context(), openapi.StartOrResumeAttemptRequestObject{Id: uuid.New()})
			if err != nil {
				t.Fatal(err)
			}
			if err := out.VisitStartOrResumeAttemptResponse(w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/app/assignments/x/attempts", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func refusedResultIn(t *testing.T, acceptLanguage string, outcome error) *httptest.ResponseRecorder {
	t.Helper()
	student := uuid.New()
	app := &application.Application{Queries: application.Queries{Result: cqrs.HandlerFunc[query.Result, domain.Result](func(context.Context, query.Result) (domain.Result, error) {
		return domain.Result{}, outcome
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)

	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: student.String()}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out, err := transport.GetAttemptResult(r.Context(), openapi.GetAttemptResultRequestObject{Id: uuid.New()})
			if err != nil {
				t.Fatal(err)
			}
			if err := out.VisitGetAttemptResultResponse(w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodGet, "/app/attempts/x/result", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAStartRefusalSpeaksTheStudentsLanguage(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome error
		status  int
		code    openapi.ErrorCode
		vi      string
		en      string
	}{
		{"forbidden", domain.ErrForbidden, http.StatusForbidden, openapi.FORBIDDEN,
			"Bạn không có quyền làm bài này.", "You do not have access to this test."},
		{"missing assignment", domain.ErrNotFound, http.StatusForbidden, openapi.FORBIDDEN,
			"Bạn không có quyền làm bài này.", "You do not have access to this test."},
		{"not open", domain.ErrAssignmentClosed, http.StatusConflict, openapi.ASSIGNMENTNOTOPEN,
			"Bài thi này hiện không mở.", "This test is not open right now."},
		{"limit reached", domain.ErrLimitReached, http.StatusConflict, openapi.ATTEMPTLIMITREACHED,
			"Bạn đã dùng hết số lượt làm bài.", "You have used all your attempts."},
		{"attempt closed", domain.ErrAttemptClosed, http.StatusConflict, openapi.ATTEMPTCLOSED,
			"Bài làm này đã kết thúc.", "This attempt has ended."},
	} {
		for _, asked := range []struct {
			name           string
			acceptLanguage string
			message        string
		}{
			{"no header", "", test.vi},
			{"english", "en-GB,en;q=0.9", test.en},
			{"neither language", "fr", test.vi},
		} {
			t.Run(test.name+"/"+asked.name, func(t *testing.T) {
				response := refusedStartIn(t, asked.acceptLanguage, test.outcome)
				if response.Code != test.status {
					t.Fatalf("status %d, want %d: %s", response.Code, test.status, response.Body.String())
				}
				var body refusal
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("%v in %s", err, response.Body.String())
				}
				if body.Error.Code != string(test.code) {
					t.Errorf("code %q, want %s", body.Error.Code, test.code)
				}
				if body.Error.Message != asked.message {
					t.Errorf("message %q, want %q", body.Error.Message, asked.message)
				}
			})
		}
	}
}

func TestAResultThatCannotBeShownSpeaksTheStudentsLanguage(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome error
		code    openapi.ErrorCode
		vi      string
		en      string
	}{
		{"in progress", domain.ErrAttemptInProgress, openapi.ATTEMPTINPROGRESS,
			"Bài chưa được nộp.", "This test has not been submitted yet."},
		{"voided", domain.ErrAttemptVoided, openapi.ATTEMPTVOIDED,
			"Lượt làm này đã bị huỷ.", "This attempt was voided."},
	} {
		for _, asked := range []struct {
			name           string
			acceptLanguage string
			message        string
		}{
			{"no header", "", test.vi},
			{"english", "en-GB,en;q=0.9", test.en},
			{"neither language", "fr", test.vi},
		} {
			t.Run(test.name+"/"+asked.name, func(t *testing.T) {
				response := refusedResultIn(t, asked.acceptLanguage, test.outcome)
				if response.Code != http.StatusConflict {
					t.Fatalf("status %d, want 409: %s", response.Code, response.Body.String())
				}
				var body refusal
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("%v in %s", err, response.Body.String())
				}
				if body.Error.Code != string(test.code) {
					t.Errorf("code %q, want %s", body.Error.Code, test.code)
				}
				if body.Error.Message != asked.message {
					t.Errorf("message %q, want %q", body.Error.Message, asked.message)
				}
			})
		}
	}
}
