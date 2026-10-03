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

func startDuringMaintenance(t *testing.T, acceptLanguage string) (*httptest.ResponseRecorder, domain.MaintenanceWindow) {
	t.Helper()
	window := domain.MaintenanceWindow{
		StartsAt: time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 10, 1, 23, 30, 0, 0, time.UTC),
	}
	student := uuid.New()
	app := &application.Application{Commands: application.Commands{StartOrResume: cqrs.HandlerFunc[command.StartOrResume, domain.Session](func(context.Context, command.StartOrResume) (domain.Session, error) {
		return domain.Session{}, &domain.MaintenanceScheduledError{Window: window}
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
	return response, window
}

type refusal struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details struct {
			StartsAt time.Time `json:"startsAt"`
			EndsAt   time.Time `json:"endsAt"`
		} `json:"details"`
	} `json:"error"`
}

func TestAStartIntoMaintenanceAnswersWithTheWindow(t *testing.T) {
	response, window := startDuringMaintenance(t, "")
	if response.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", response.Code, response.Body.String())
	}
	var body refusal
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	switch {
	case body.Error.Code != string(openapi.MAINTENANCESCHEDULED):
		t.Errorf("code %q, want MAINTENANCE_SCHEDULED", body.Error.Code)
	case !body.Error.Details.StartsAt.Equal(window.StartsAt), !body.Error.Details.EndsAt.Equal(window.EndsAt):
		t.Errorf("details %+v, want the window %+v", body.Error.Details, window)
	case body.Error.Message != "Quizzivy sắp được cập nhật trước khi bài làm kết thúc. Hãy bắt đầu sau khi cập nhật xong.":
		t.Errorf("message %q, want Vietnamese by default", body.Error.Message)
	}
}

func TestAStartIntoMaintenanceSpeaksTheStudentsLanguage(t *testing.T) {
	response, _ := startDuringMaintenance(t, "en-GB,en;q=0.9")
	var body refusal
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Message != "Quizzivy will be updated before this attempt would end. Start it once the update is over." {
		t.Errorf("message %q, want English for an English browser", body.Error.Message)
	}
}
