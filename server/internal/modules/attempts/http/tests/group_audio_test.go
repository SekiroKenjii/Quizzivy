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

func TestSharedAudioTransportPreservesGestureIdentityAndSessionErrors(t *testing.T) {
	attempt, student, session, recording, play := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, test := range []struct {
		name   string
		cause  error
		status int
		code   string
	}{
		{"counted", nil, 200, ""},
		{"forbidden", domain.ErrForbidden, 403, "FORBIDDEN"},
		{"closed", domain.ErrAttemptClosed, 409, "ATTEMPT_CLOSED"},
		{"session", domain.ErrSessionSuperseded, 409, "SESSION_SUPERSEDED"},
		{"deadline", domain.ErrDeadlinePassed, 409, "DEADLINE_PASSED"},
		{"gesture conflict", domain.ErrPlayIDConflict, 409, "PLAY_ID_CONFLICT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := &application.Application{Commands: application.Commands{RecordGroupPlay: cqrs.HandlerFunc[command.RecordGroupPlay, domain.GroupPlays](func(_ context.Context, cmd command.RecordGroupPlay) (domain.GroupPlays, error) {
				want := domain.GroupPlayInput{AttemptID: attempt.String(), StudentID: student.String(), SessionID: session.String(), RecordingID: recording.String(), PlayID: play.String()}
				if cmd.Input != want {
					t.Fatalf("gesture identity changed: %+v", cmd.Input)
				}
				maxPlays := 2
				return domain.GroupPlays{PlayID: play.String(), Plays: 3, MaxPlays: &maxPlays}, test.cause
			})}}
			transport := attemptshttp.NewAttempts(app, nil, nil, nil)
			handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
				return httpx.Principal{UserID: student.String(), Role: "student"}, nil
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response, err := transport.RecordGroupAudioPlay(r.Context(), openapi.RecordGroupAudioPlayRequestObject{Id: attempt, Body: &openapi.GroupAudioPlayInput{SessionId: session, RecordingId: recording, PlayId: play}})
				if err != nil {
					t.Fatal(err)
				}
				if err := response.VisitRecordGroupAudioPlayResponse(w); err != nil {
					t.Fatal(err)
				}
			}))
			request := httptest.NewRequest(http.MethodPost, "/app/attempts/"+attempt.String()+"/group-audio-play", nil)
			request.Header.Set("Authorization", "Bearer fixture")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status %d, want %d", response.Code, test.status)
			}
			if test.cause == nil {
				var body openapi.GroupAudioPlayResult
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.PlayId != play || body.Plays != 3 || body.MaxPlays == nil || *body.MaxPlays != 2 {
					t.Fatalf("response: %s, %v", response.Body.String(), err)
				}
			} else {
				var body openapi.ErrorResponse
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || string(body.Error.Code) != test.code {
					t.Fatalf("response: %s, %v", response.Body.String(), err)
				}
			}
		})
	}
}
