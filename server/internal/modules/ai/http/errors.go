package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"
)

// retryAfterSeconds is what a caller whose AI call produced nothing is told to wait.
const retryAfterSeconds = 30

type failure struct {
	status     int
	retryAfter int
	body       openapi.ErrorResponse
}

func (f *failure) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if f.retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(f.retryAfter))
	}
	w.WriteHeader(f.status)
	return json.NewEncoder(w).Encode(f.body)
}

func (f *failure) VisitGenerateAiQuestionsResponse(w http.ResponseWriter) error  { return f.write(w) }
func (f *failure) VisitCheckTestWithAiResponse(w http.ResponseWriter) error      { return f.write(w) }
func (f *failure) VisitMakeTestVariantResponse(w http.ResponseWriter) error      { return f.write(w) }
func (f *failure) VisitWriteAiExplanationResponse(w http.ResponseWriter) error   { return f.write(w) }
func (f *failure) VisitSuggestAiDistractorsResponse(w http.ResponseWriter) error { return f.write(w) }
func (f *failure) VisitSuggestAiScoreResponse(w http.ResponseWriter) error       { return f.write(w) }
func (f *failure) VisitExplainAssignmentResultsResponse(w http.ResponseWriter) error {
	return f.write(w)
}
func (f *failure) VisitCleanUpQuestionBankResponse(w http.ResponseWriter) error    { return f.write(w) }
func (f *failure) VisitGetAiUsageResponse(w http.ResponseWriter) error             { return f.write(w) }
func (f *failure) VisitRequestAiCreditsResponse(w http.ResponseWriter) error       { return f.write(w) }
func (f *failure) VisitGetAiCreditsResponse(w http.ResponseWriter) error           { return f.write(w) }
func (f *failure) VisitUpdateAiRulesResponse(w http.ResponseWriter) error          { return f.write(w) }
func (f *failure) VisitAddAiCreditsResponse(w http.ResponseWriter) error           { return f.write(w) }
func (f *failure) VisitSetAiAllowanceResponse(w http.ResponseWriter) error         { return f.write(w) }
func (f *failure) VisitApproveAiCreditRequestResponse(w http.ResponseWriter) error { return f.write(w) }
func (f *failure) VisitDeclineAiCreditRequestResponse(w http.ResponseWriter) error { return f.write(w) }

type translation struct {
	err     error
	status  int
	code    openapi.ErrorCode
	message string
}

func translations(ctx context.Context) []translation {
	return []translation{
		{domain.ErrOff, http.StatusNotImplemented, openapi.AIOFF, httpx.Text(ctx, "Trợ lý AI chưa được bật trên máy chủ này.", "The AI assistant is not turned on on this server.")},
		{domain.ErrPaused, http.StatusForbidden, openapi.AIPAUSED, httpx.Text(ctx, "AI đang bị tạm dừng cho tài khoản của bạn.", "AI is paused for your account.")},
		{domain.ErrReadOff, http.StatusForbidden, openapi.AIREADOFF, httpx.Text(ctx, "Bạn đã tắt việc cho AI đọc bài làm của học viên. Bật lại trong Cài đặt › Trợ lý AI.", "Letting AI read student answers is off. Turn it on in Settings › AI assistant.")},
		{domain.ErrMissingPermission, http.StatusForbidden, openapi.FORBIDDEN, httpx.Text(ctx, "Bạn không có quyền thực hiện thao tác này.", "You do not have permission to do this.")},
		{domain.ErrForbidden, http.StatusForbidden, openapi.FORBIDDEN, httpx.Text(ctx, "Bạn không có quyền thao tác trên tài khoản này.", "You do not have permission to act on this account.")},
		{domain.ErrNotFound, http.StatusNotFound, openapi.NOTFOUND, httpx.Text(ctx, "Không tìm thấy dữ liệu.", "The data was not found.")},
		{domain.ErrNotATeacher, http.StatusNotFound, openapi.NOTFOUND, httpx.Text(ctx, "Không tìm thấy giáo viên này.", "No such teacher.")},
		{domain.ErrRequestNotFound, http.StatusNotFound, openapi.NOTFOUND, httpx.Text(ctx, "Không tìm thấy yêu cầu này.", "No such request.")},
		{domain.ErrNothingToRead, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Chưa có nội dung nào để AI đọc.", "There is nothing for the assistant to read yet.")},
		{domain.ErrTooLarge, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Nội dung quá dài để AI đọc trong một lần.", "There is too much to read in one go.")},
		{domain.ErrNotChoice, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Chỉ gợi ý đáp án sai cho câu hỏi một hoặc nhiều lựa chọn.", "Wrong options can be suggested for single and multiple choice questions only.")},
		{domain.ErrNoSample, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Câu tự luận chưa có đáp án mẫu.", "The essay has no sample answer yet.")},
		{domain.ErrNoKey, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Câu hỏi chưa có đáp án đúng.", "The question has no correct answer yet.")},
		{domain.ErrNotManual, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Chỉ gợi ý điểm cho câu trả lời ngắn chấm tay.", "Scores are suggested for short answers marked by hand only.")},
		{domain.ErrUnanswered, http.StatusUnprocessableEntity, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Học viên chưa trả lời câu này.", "The student left this question unanswered.")},
		{domain.ErrAttemptInProgress, http.StatusConflict, openapi.ATTEMPTINPROGRESS, httpx.Text(ctx, "Bài làm chưa được nộp nên chưa có gì để chấm.", "The attempt has not been handed in, so there is nothing to grade yet.")},
		{domain.ErrAttemptVoided, http.StatusConflict, openapi.ATTEMPTVOIDED, httpx.Text(ctx, "Bài làm đã bị huỷ nên không được tính.", "The attempt was voided and does not count.")},
		{domain.ErrRequestsOff, http.StatusConflict, openapi.AIREQUESTSOFF, httpx.Text(ctx, "Trung tâm không nhận yêu cầu thêm tín dụng AI.", "The school does not take requests for more AI credits.")},
		{domain.ErrRequestPending, http.StatusConflict, openapi.AIREQUESTPENDING, httpx.Text(ctx, "Bạn đã có một yêu cầu đang chờ duyệt trong tháng này.", "You already have a request waiting this month.")},
		{domain.ErrRequestDecided, http.StatusConflict, openapi.AIREQUESTDECIDED, httpx.Text(ctx, "Yêu cầu này đã được xử lý.", "This request was decided already.")},
		{domain.ErrRequestExpired, http.StatusConflict, openapi.AIREQUESTEXPIRED, httpx.Text(ctx, "Yêu cầu thuộc về tháng trước nên không thể duyệt.", "The request is from an earlier month and cannot be approved.")},
		{domain.ErrAllowanceBelowUse, http.StatusConflict, openapi.AIALLOWANCEBELOWUSE, httpx.Text(ctx, "Hạn mức không được thấp hơn mức giáo viên đã dùng hoặc đang giữ trong tháng.", "The allowance cannot be below what the teacher has used or holds this month.")},
	}
}

func failureOf(ctx context.Context, err error) (*failure, error) {
	var exhausted *domain.Exhausted
	var invalid *validation.Error
	switch {
	case errors.As(err, &exhausted):
		return exhaustedFailure(ctx, exhausted.Scope), nil
	case errors.Is(err, domain.ErrUnavailable):
		message := httpx.Text(ctx, "AI chưa phản hồi. Hãy thử lại sau ít phút.", "AI didn’t respond. Try again in a minute.")
		return &failure{status: http.StatusServiceUnavailable, retryAfter: retryAfterSeconds, body: httpapi.Error(ctx, openapi.AIUNAVAILABLE, message)}, nil
	case errors.As(err, &invalid):
		return &failure{status: http.StatusBadRequest, body: httpapi.Invalid(ctx, httpx.Text(ctx, "Dữ liệu không hợp lệ.", "The data is not valid."), invalid)}, nil
	}
	for _, t := range translations(ctx) {
		if errors.Is(err, t.err) {
			return &failure{status: t.status, body: httpapi.Error(ctx, t.code, t.message)}, nil
		}
	}
	return nil, err
}

func exhaustedFailure(ctx context.Context, scope domain.Scope) *failure {
	message := httpx.Text(ctx, "Hạn mức AI của bạn đã dùng hết. Hãy xin thêm trong Cài đặt.", "Your allowance is used up. Ask for more in Settings.")
	if scope == domain.ScopePool {
		message = httpx.Text(ctx, "Quỹ AI của trung tâm đã dùng hết trong tháng này.", "The school’s AI pool is used up this month.")
	}
	body := httpapi.ErrorWithDetails(ctx, openapi.AICREDITSEXHAUSTED, message, map[string]interface{}{"scope": string(scope)})
	return &failure{status: http.StatusPaymentRequired, body: body}
}
