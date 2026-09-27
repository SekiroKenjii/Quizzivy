package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

func serve(t *testing.T, student uuid.UUID, handle func(w http.ResponseWriter, r *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: student.String(), Role: "student"}, nil
	})(http.HandlerFunc(handle))
	request := httptest.NewRequest(http.MethodPost, "/app/attempts/x", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestTheAutosaveAnswerCarriesTheDeadline(t *testing.T) {
	attempt, student, session := uuid.New(), uuid.New(), uuid.New()
	deadline := time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC)
	app := &application.Application{Commands: application.Commands{Save: cqrs.HandlerFunc[command.Save, domain.SaveResult](func(context.Context, command.Save) (domain.SaveResult, error) {
		return domain.SaveResult{SavedAt: deadline.Add(-time.Hour), DeadlineAt: deadline}, nil
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)

	response := serve(t, student, func(w http.ResponseWriter, r *http.Request) {
		out, err := transport.SaveAnswers(r.Context(), openapi.SaveAnswersRequestObject{Id: attempt, Body: &openapi.SaveAnswersJSONRequestBody{SessionId: session}})
		if err != nil {
			t.Fatal(err)
		}
		if err := out.VisitSaveAnswersResponse(w); err != nil {
			t.Fatal(err)
		}
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		DeadlineAt time.Time `json:"deadlineAt"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !body.DeadlineAt.Equal(deadline) {
		t.Fatalf("deadlineAt in %s, want %v (%v)", response.Body.String(), deadline, err)
	}
}

func TestAnEarlyTimerAnswersWithTheDeadlineToWaitFor(t *testing.T) {
	attempt, student := uuid.New(), uuid.New()
	deadline := time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC)
	app := &application.Application{Commands: application.Commands{Submit: cqrs.HandlerFunc[command.Submit, domain.Attempt](func(_ context.Context, cmd command.Submit) (domain.Attempt, error) {
		if cmd.Reason != domain.TimerExpired {
			t.Fatalf("reason %s reached the command, want timer_expired", cmd.Reason)
		}
		return domain.Attempt{}, &domain.DeadlineNotReachedError{DeadlineAt: deadline}
	})}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)
	reason := openapi.SubmitAttemptJSONBodyReason("timer_expired")

	response := serve(t, student, func(w http.ResponseWriter, r *http.Request) {
		out, err := transport.SubmitAttempt(r.Context(), openapi.SubmitAttemptRequestObject{Id: attempt, Body: &openapi.SubmitAttemptJSONRequestBody{Reason: &reason}})
		if err != nil {
			t.Fatal(err)
		}
		if err := out.VisitSubmitAttemptResponse(w); err != nil {
			t.Fatal(err)
		}
	})
	if response.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var body openapi.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != openapi.DEADLINENOTREACHED {
		t.Errorf("code %s, want DEADLINE_NOT_REACHED", body.Error.Code)
	}
	if body.Error.Details == nil {
		t.Fatalf("no details in %s", response.Body.String())
	}
	got, err := time.Parse(time.RFC3339Nano, (*body.Error.Details)["deadlineAt"].(string))
	if err != nil || !got.Equal(deadline) {
		t.Errorf("details.deadlineAt = %v (%v), want %v", (*body.Error.Details)["deadlineAt"], err, deadline)
	}
}
