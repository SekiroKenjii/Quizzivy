package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/domain"
	aihttp "quizzivy/internal/modules/ai/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"
)

type store struct {
	domain.Repository
	fail error
}

func (s store) SetAllowance(context.Context, domain.SetAllowanceInput) (domain.TeacherMonth, error) {
	if s.fail != nil {
		return domain.TeacherMonth{}, s.fail
	}
	return domain.TeacherMonth{Allowance: 20_000}, nil
}

func (s store) Rules(context.Context) (domain.Rules, error) {
	return domain.Rules{WarnPercent: 80}, nil
}

type nobodyHoldsMore struct{}

func (nobodyHoldsMore) Preferences(context.Context, string) (domain.Preferences, error) {
	return domain.Preferences{Language: domain.English, ReadStudentAnswers: true}, nil
}

func (nobodyHoldsMore) Permissions(context.Context, string) (access.Set, error) {
	return access.Set{}, nil
}

func pauseWith(t *testing.T, enabled bool, acceptLanguage string, fail error) *httptest.ResponseRecorder {
	t.Helper()
	app := application.New(store{fail: fail}, application.Options{People: nobodyHoldsMore{}})
	transport := aihttp.NewAI(app, enabled)
	admin := uuid.NewString()
	paused := true

	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: admin, Access: access.Principal{UserID: admin, Permissions: access.NewSet(access.AICreditsManage)}}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out, err := transport.SetAiAllowance(r.Context(), openapi.SetAiAllowanceRequestObject{
				UserId: uuid.New(), Body: &openapi.SetAiAllowanceJSONRequestBody{Paused: &paused},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := out.VisitSetAiAllowanceResponse(w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPut, "/admin/ai/teachers/x/allowance", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func envelope(t *testing.T, response *httptest.ResponseRecorder) (code, message string, details map[string]any) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an error envelope: %s", response.Body.String())
	}
	return body.Error.Code, body.Error.Message, body.Error.Details
}

func TestEveryRefusalAnswersItsStatusItsCodeAndTheCallersLanguage(t *testing.T) {
	for _, c := range []struct {
		name   string
		err    error
		status int
		code   string
		vi, en string
	}{
		{"a teacher who is paused", domain.ErrPaused, 403, "AI_PAUSED", "AI đang bị tạm dừng cho tài khoản của bạn.", "AI is paused for your account."},
		{"answers turned off", domain.ErrReadOff, 403, "AI_READ_OFF", "Bạn đã tắt việc cho AI đọc bài làm của học viên. Bật lại trong Cài đặt › Trợ lý AI.", "Letting AI read student answers is off. Turn it on in Settings › AI assistant."},
		{"a teacher the author may not act on", domain.ErrForbidden, 403, "FORBIDDEN", "Bạn không có quyền thao tác trên tài khoản này.", "You do not have permission to act on this account."},
		{"a missing permission", domain.ErrMissingPermission, 403, "FORBIDDEN", "Bạn không có quyền thực hiện thao tác này.", "You do not have permission to do this."},
		{"data that is not found", domain.ErrNotFound, 404, "NOT_FOUND", "Không tìm thấy dữ liệu.", "The data was not found."},
		{"a user who is not a teacher", domain.ErrNotATeacher, 404, "NOT_FOUND", "Không tìm thấy giáo viên này.", "No such teacher."},
		{"a request that is none", domain.ErrRequestNotFound, 404, "NOT_FOUND", "Không tìm thấy yêu cầu này.", "No such request."},
		{"nothing to read", domain.ErrNothingToRead, 422, "VALIDATION_FAILED", "Chưa có nội dung nào để AI đọc.", "There is nothing for the assistant to read yet."},
		{"too much to read", domain.ErrTooLarge, 422, "VALIDATION_FAILED", "Nội dung quá dài để AI đọc trong một lần.", "There is too much to read in one go."},
		{"a question that is not a choice", domain.ErrNotChoice, 422, "VALIDATION_FAILED", "Chỉ gợi ý đáp án sai cho câu hỏi một hoặc nhiều lựa chọn.", "Wrong options can be suggested for single and multiple choice questions only."},
		{"a question with no key", domain.ErrNoKey, 422, "VALIDATION_FAILED", "Câu hỏi chưa có đáp án đúng.", "The question has no correct answer yet."},
		{"a question not marked by hand", domain.ErrNotManual, 422, "VALIDATION_FAILED", "Chỉ gợi ý điểm cho câu trả lời ngắn chấm tay.", "Scores are suggested for short answers marked by hand only."},
		{"an unanswered question", domain.ErrUnanswered, 422, "VALIDATION_FAILED", "Học viên chưa trả lời câu này.", "The student left this question unanswered."},
		{"an unfinished attempt", domain.ErrAttemptInProgress, 409, "ATTEMPT_IN_PROGRESS", "Bài làm chưa được nộp nên chưa có gì để chấm.", "The attempt has not been handed in, so there is nothing to grade yet."},
		{"a voided attempt", domain.ErrAttemptVoided, 409, "ATTEMPT_VOIDED", "Bài làm đã bị huỷ nên không được tính.", "The attempt was voided and does not count."},
		{"requests turned off", domain.ErrRequestsOff, 409, "AI_REQUESTS_OFF", "Trung tâm không nhận yêu cầu thêm tín dụng AI.", "The school does not take requests for more AI credits."},
		{"a request waiting", domain.ErrRequestPending, 409, "AI_REQUEST_PENDING", "Bạn đã có một yêu cầu đang chờ duyệt trong tháng này.", "You already have a request waiting this month."},
		{"a request decided", domain.ErrRequestDecided, 409, "AI_REQUEST_DECIDED", "Yêu cầu này đã được xử lý.", "This request was decided already."},
		{"a request from an earlier month", domain.ErrRequestExpired, 409, "AI_REQUEST_EXPIRED", "Yêu cầu thuộc về tháng trước nên không thể duyệt.", "The request is from an earlier month and cannot be approved."},
		{"an allowance below the use", domain.ErrAllowanceBelowUse, 409, "AI_ALLOWANCE_BELOW_USE", "Hạn mức không được thấp hơn mức giáo viên đã dùng hoặc đang giữ trong tháng.", "The allowance cannot be below what the teacher has used or holds this month."},
		{"a provider that did not answer", fmt.Errorf("%w: down", domain.ErrUnavailable), 503, "AI_UNAVAILABLE", "AI chưa phản hồi. Hãy thử lại sau ít phút.", "AI didn’t respond. Try again in a minute."},
	} {
		for language, want := range map[string]string{"vi": c.vi, "en": c.en} {
			response := pauseWith(t, true, language, c.err)
			code, message, _ := envelope(t, response)
			if response.Code != c.status || code != c.code || message != want {
				t.Errorf("%s in %s: %d %s %q, want %d %s %q", c.name, language, response.Code, code, message, c.status, c.code, want)
			}
		}
	}
}

func TestAProviderThatDidNotAnswerAsksTheCallerToWaitHalfAMinute(t *testing.T) {
	response := pauseWith(t, true, "en", domain.ErrUnavailable)
	if got := response.Header().Get("Retry-After"); got != "30" {
		t.Errorf("Retry-After %q, want 30", got)
	}
	if got := pauseWith(t, true, "en", domain.ErrNotFound).Header().Get("Retry-After"); got != "" {
		t.Errorf("a refusal that retrying cannot cure asks the caller to wait %q", got)
	}
}

func TestARefusedReservationNamesTheBudgetThatCouldNotCoverIt(t *testing.T) {
	for scope, message := range map[domain.Scope]string{
		domain.ScopeTeacher: "Your allowance is used up. Ask for more in Settings.",
		domain.ScopePool:    "The school’s AI pool is used up this month.",
	} {
		response := pauseWith(t, true, "en", &domain.Exhausted{Scope: scope})
		code, got, details := envelope(t, response)
		if response.Code != http.StatusPaymentRequired || code != "AI_CREDITS_EXHAUSTED" || got != message || details["scope"] != string(scope) {
			t.Errorf("scope %s: %d %s %q %v, want 402 naming the scope with %q", scope, response.Code, code, got, details, message)
		}
	}
}

func TestAnInvalidFieldAnswers400WithItsFieldAndAnUnknownFailureIsNotMasked(t *testing.T) {
	var invalid validation.Error
	invalid.Add("credits", "outside the allowance a teacher may have")
	response := pauseWith(t, true, "en", invalid.OrNil())
	if code, _, _ := envelope(t, response); response.Code != http.StatusBadRequest || code != "VALIDATION_FAILED" {
		t.Errorf("a validation error answered %d %s", response.Code, code)
	}

	app := application.New(store{fail: errors.New("the store is down")}, application.Options{People: nobodyHoldsMore{}})
	transport := aihttp.NewAI(app, true)
	admin := uuid.NewString()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: admin, Access: access.Principal{UserID: admin}}, nil
		})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			paused := true
			_, err := transport.SetAiAllowance(r.Context(), openapi.SetAiAllowanceRequestObject{UserId: uuid.New(), Body: &openapi.SetAiAllowanceJSONRequestBody{Paused: &paused}})
			if err == nil {
				t.Error("a failure the contract does not name was answered as a refusal")
			}
		})))
	request := httptest.NewRequest(http.MethodPut, "/admin/ai/teachers/x/allowance", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	handler.ServeHTTP(httptest.NewRecorder(), request)
}

func TestWithNoProviderEveryOperationAnswers501BeforeItReadsAnything(t *testing.T) {
	response := pauseWith(t, false, "vi", nil)
	code, message, _ := envelope(t, response)
	if response.Code != http.StatusNotImplemented || code != "AI_OFF" || message != "Trợ lý AI chưa được bật trên máy chủ này." {
		t.Errorf("an assistant that is off answered %d %s %q", response.Code, code, message)
	}
}
