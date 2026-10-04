package http_test

import (
	"context"
	"encoding/json"
	"maps"
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
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type refusing func(ctx context.Context, w http.ResponseWriter) error

func refusedIn(t *testing.T, acceptLanguage string, serve refusing) *httptest.ResponseRecorder {
	t.Helper()
	student := uuid.NewString()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: student}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/app/attempts/x", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func teacherRefusedIn(t *testing.T, acceptLanguage string, serve refusing) *httptest.ResponseRecorder {
	t.Helper()
	teacher := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingGrading, access.TeachingAttemptsIntervene)}
	gate := httpx.RequirePermission(map[string]access.Requirement{"POST /teacher/attempts": access.AnyOf(access.WorkspaceTeacher)},
		reachResolver{teacher.UserID: teacher})
	reached := false
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: teacher.UserID}, nil
		})(gate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		}))))
	request := httptest.NewRequest(http.MethodPost, "/teacher/attempts", nil)
	request.Pattern = "POST /teacher/attempts"
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !reached {
		t.Fatalf("the gate did not pass the request: %d %s", response.Code, response.Body.String())
	}
	return response
}

func saving(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Save: cqrs.HandlerFunc[command.Save, domain.SaveResult](func(context.Context, command.Save) (domain.SaveResult, error) {
			return domain.SaveResult{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.SaveAnswers(ctx, openapi.SaveAnswersRequestObject{Id: uuid.New(), Body: &openapi.SaveAnswersJSONRequestBody{SessionId: uuid.New()}})
		if err != nil {
			return err
		}
		return response.VisitSaveAnswersResponse(w)
	}
}

func submitting(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Submit: cqrs.HandlerFunc[command.Submit, domain.Attempt](func(context.Context, command.Submit) (domain.Attempt, error) {
			return domain.Attempt{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		reason := openapi.SubmitAttemptJSONBodyReason("timer_expired")
		response, err := transport.SubmitAttempt(ctx, openapi.SubmitAttemptRequestObject{Id: uuid.New(), Body: &openapi.SubmitAttemptJSONRequestBody{Reason: &reason}})
		if err != nil {
			return err
		}
		return response.VisitSubmitAttemptResponse(w)
	}
}

func playingSharedAudio(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		RecordGroupPlay: cqrs.HandlerFunc[command.RecordGroupPlay, domain.GroupPlays](func(context.Context, command.RecordGroupPlay) (domain.GroupPlays, error) {
			return domain.GroupPlays{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.RecordGroupAudioPlay(ctx, openapi.RecordGroupAudioPlayRequestObject{Id: uuid.New(), Body: &openapi.GroupAudioPlayInput{SessionId: uuid.New(), RecordingId: uuid.New(), PlayId: uuid.New()}})
		if err != nil {
			return err
		}
		return response.VisitRecordGroupAudioPlayResponse(w)
	}
}

func flushing(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Flush: cqrs.HandlerFunc[command.Flush, cqrs.Nothing](func(context.Context, command.Flush) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.FlushEvents(ctx, openapi.FlushEventsRequestObject{Id: uuid.New(), JSONBody: &openapi.FlushEventsJSONRequestBody{SessionId: uuid.New(), Events: []openapi.IntegrityEventInput{}}})
		if err != nil {
			return err
		}
		return response.VisitFlushEventsResponse(w)
	}
}

func beaconing(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Flush: cqrs.HandlerFunc[command.Flush, cqrs.Nothing](func(context.Context, command.Flush) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		beacon := `{"beaconToken":"fixture","sessionId":"` + uuid.NewString() + `","events":[]}`
		response, err := transport.FlushEvents(ctx, openapi.FlushEventsRequestObject{Id: uuid.New(), TextBody: &beacon})
		if err != nil {
			return err
		}
		return response.VisitFlushEventsResponse(w)
	}
}

func extending(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Extend: cqrs.HandlerFunc[command.Extend, domain.Attempt](func(context.Context, command.Extend) (domain.Attempt, error) {
			return domain.Attempt{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.ExtendAttempt(ctx, openapi.ExtendAttemptRequestObject{Id: uuid.New(), Body: &openapi.ExtendAttemptJSONRequestBody{Minutes: 5, Reason: "thêm"}})
		if err != nil {
			return err
		}
		return response.VisitExtendAttemptResponse(w)
	}
}

func grading(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Grade: cqrs.HandlerFunc[command.Grade, domain.Score](func(context.Context, command.Grade) (domain.Score, error) {
			return domain.Score{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GradeAttempt(ctx, openapi.GradeAttemptRequestObject{Id: uuid.New(), Body: &openapi.GradeAttemptJSONRequestBody{}})
		if err != nil {
			return err
		}
		return response.VisitGradeAttemptResponse(w)
	}
}

func finishingGrading(outcome error) refusing {
	transport := attemptshttp.NewAttempts(&application.Application{Commands: application.Commands{
		Finish: cqrs.HandlerFunc[command.Finish, domain.Attempt](func(context.Context, command.Finish) (domain.Attempt, error) {
			return domain.Attempt{}, outcome
		}),
	}}, nil, nil, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.FinishGrading(ctx, openapi.FinishGradingRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitFinishGradingResponse(w)
	}
}

func TestAttemptRefusalsSpeakTheCallersLanguage(t *testing.T) {
	const (
		unwrittenVI = "Không ghi được nhật ký cho bài làm này."
		unwrittenEN = "The log for this attempt could not be written."
	)
	deadline := time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC)
	waitFor := map[string]interface{}{"deadlineAt": "2026-10-01T15:30:00Z"}
	aboveCeiling, unanswered, notOnPaper := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, c := range []struct {
		name      string
		in        func(t *testing.T, acceptLanguage string, serve refusing) *httptest.ResponseRecorder
		serve     refusing
		status    int
		code      openapi.ErrorCode
		vi        string
		en        string
		detailsVI map[string]interface{}
		detailsEN map[string]interface{}
	}{
		{
			name:   "a save from a session another one replaced",
			in:     refusedIn,
			serve:  saving(domain.ErrSessionSuperseded),
			status: http.StatusConflict,
			code:   openapi.SESSIONSUPERSEDED,
			vi:     "Bài làm này đã được mở ở nơi khác.",
			en:     "This attempt was opened somewhere else.",
		},
		{
			name:   "a save after the deadline",
			in:     refusedIn,
			serve:  saving(domain.ErrDeadlinePassed),
			status: http.StatusConflict,
			code:   openapi.DEADLINEPASSED,
			vi:     "Đã hết giờ làm bài.",
			en:     "Time is up.",
		},
		{
			name:   "a save into an attempt that has ended",
			in:     refusedIn,
			serve:  saving(domain.ErrAttemptClosed),
			status: http.StatusConflict,
			code:   openapi.ATTEMPTCLOSED,
			vi:     "Bài làm này đã kết thúc.",
			en:     "This attempt has ended.",
		},
		{
			name:      "a timer's submit before the extended deadline",
			in:        refusedIn,
			serve:     submitting(&domain.DeadlineNotReachedError{DeadlineAt: deadline}),
			status:    http.StatusConflict,
			code:      openapi.DEADLINENOTREACHED,
			vi:        "Chưa hết giờ: thời gian làm bài đã được gia hạn.",
			en:        "Time is not up yet: the attempt has been extended.",
			detailsVI: waitFor,
			detailsEN: waitFor,
		},
		{
			name:   "a submit of an attempt already handed in",
			in:     refusedIn,
			serve:  submitting(domain.ErrAttemptClosed),
			status: http.StatusConflict,
			code:   openapi.ATTEMPTCLOSED,
			vi:     "Bài làm này đã được nộp.",
			en:     "This attempt has already been submitted.",
		},
		{
			name:   "a play id that belongs to another recording",
			in:     refusedIn,
			serve:  playingSharedAudio(domain.ErrPlayIDConflict),
			status: http.StatusConflict,
			code:   openapi.PLAYIDCONFLICT,
			vi:     "Mã lượt nghe đã được dùng cho bản ghi khác. Vui lòng tải lại bài làm.",
			en:     "This play id was already used for another recording. Please reload the attempt.",
		},
		{
			name:   "extending an attempt that has ended",
			in:     teacherRefusedIn,
			serve:  extending(domain.ErrAttemptClosed),
			status: http.StatusConflict,
			code:   openapi.ATTEMPTCLOSED,
			vi:     "Lượt làm này đã kết thúc nên không gia hạn được.",
			en:     "This attempt has ended, so it cannot be extended.",
		},
		{
			name:   "extending a voided attempt",
			in:     teacherRefusedIn,
			serve:  extending(domain.ErrAttemptVoided),
			status: http.StatusConflict,
			code:   openapi.ATTEMPTVOIDED,
			vi:     "Lượt làm này đã bị huỷ.",
			en:     "This attempt was voided.",
		},
		{
			name: "grading with marks that cannot be kept",
			in:   teacherRefusedIn,
			serve: grading(&domain.GradeValidationError{Items: []domain.GradeItemError{
				{QuestionID: aboveCeiling, Reason: "above_ceiling"},
				{QuestionID: unanswered, Reason: "unanswered"},
				{QuestionID: notOnPaper, Reason: "not_on_paper"},
			}}),
			status: http.StatusBadRequest,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Điểm không hợp lệ.",
			en:     "The score is not valid.",
			detailsVI: map[string]interface{}{
				aboveCeiling: "Điểm vượt quá điểm tối đa của câu.",
				unanswered:   "Học viên không trả lời câu này.",
				notOnPaper:   "Câu này không có trong đề.",
			},
			detailsEN: map[string]interface{}{
				aboveCeiling: "The score is above the question's maximum.",
				unanswered:   "The student did not answer this question.",
				notOnPaper:   "This question is not in the test.",
			},
		},
		{
			name:   "finishing while a written answer is ungraded",
			in:     teacherRefusedIn,
			serve:  finishingGrading(domain.ErrGradingIncomplete),
			status: http.StatusConflict,
			code:   openapi.GRADINGINCOMPLETE,
			vi:     "Còn câu tự luận chưa chấm.",
			en:     "Some written answers are not graded yet.",
		},
		{
			name:   "a flush the attempt does not accept",
			in:     refusedIn,
			serve:  flushing(domain.ErrForbidden),
			status: http.StatusForbidden,
			code:   openapi.FORBIDDEN,
			vi:     unwrittenVI,
			en:     unwrittenEN,
		},
		{
			name:   "a beacon whose token has expired",
			in:     refusedIn,
			serve:  beaconing(domain.ErrBeaconExpired),
			status: http.StatusForbidden,
			code:   openapi.FORBIDDEN,
			vi:     unwrittenVI,
			en:     unwrittenEN,
		},
	} {
		for _, language := range []struct {
			accept  string
			message string
			details map[string]interface{}
		}{{"", c.vi, c.detailsVI}, {"en", c.en, c.detailsEN}} {
			response := c.in(t, language.accept, c.serve)
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s with Accept-Language %q: %v in %s", c.name, language.accept, err, response.Body.String())
			}
			if response.Code != c.status || body.Error.Code != c.code {
				t.Errorf("%s with Accept-Language %q answered %d %s, want %d %s", c.name, language.accept, response.Code, body.Error.Code, c.status, c.code)
			}
			if body.Error.Message != language.message {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, body.Error.Message, language.message)
			}
			var details map[string]interface{}
			if body.Error.Details != nil {
				details = *body.Error.Details
			}
			if !maps.Equal(details, language.details) {
				t.Errorf("%s with Accept-Language %q carries details %v, want %v", c.name, language.accept, details, language.details)
			}
		}
	}
}
