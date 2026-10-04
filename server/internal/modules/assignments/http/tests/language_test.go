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
	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

type refusing func(ctx context.Context, w http.ResponseWriter) error

func refusedIn(t *testing.T, acceptLanguage string, serve refusing) *httptest.ResponseRecorder {
	t.Helper()
	caller := uuid.NewString()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: caller}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/teacher/assignments", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func creating(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		Create: cqrs.HandlerFunc[command.Create, domain.Assignment](func(context.Context, command.Create) (domain.Assignment, error) {
			return domain.Assignment{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.CreateAssignment(ctx, openapi.CreateAssignmentRequestObject{Body: &openapi.CreateAssignmentJSONRequestBody{}})
		if err != nil {
			return err
		}
		return response.VisitCreateAssignmentResponse(w)
	}
}

func updating(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		Update: cqrs.HandlerFunc[command.Update, domain.Assignment](func(context.Context, command.Update) (domain.Assignment, error) {
			return domain.Assignment{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.UpdateAssignment(ctx, openapi.UpdateAssignmentRequestObject{Id: uuid.New(), Body: &openapi.UpdateAssignmentJSONRequestBody{}})
		if err != nil {
			return err
		}
		return response.VisitUpdateAssignmentResponse(w)
	}
}

func reopening(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		Reopen: cqrs.HandlerFunc[command.Reopen, domain.Assignment](func(context.Context, command.Reopen) (domain.Assignment, error) {
			return domain.Assignment{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.ReopenAssignment(ctx, openapi.ReopenAssignmentRequestObject{Id: uuid.New(), Body: &openapi.ReopenAssignmentJSONRequestBody{ClosesAt: time.Now().Add(time.Hour), Reason: "Gia hạn"}})
		if err != nil {
			return err
		}
		return response.VisitReopenAssignmentResponse(w)
	}
}

func deleting(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(context.Context, command.Delete) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteAssignment(ctx, openapi.DeleteAssignmentRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitDeleteAssignmentResponse(w)
	}
}

func openingAsStudent(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Queries: application.Queries{
		StudentDetail: cqrs.HandlerFunc[query.StudentDetail, domain.StudentDetail](func(context.Context, query.StudentDetail) (domain.StudentDetail, error) {
			return domain.StudentDetail{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetMyAssignment(ctx, openapi.GetMyAssignmentRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitGetMyAssignmentResponse(w)
	}
}

func TestAssignmentRefusalsSpeakTheCallersLanguage(t *testing.T) {
	const (
		blankReasonVI  = "Hãy ghi lý do mở lại."
		blankReasonEN  = "Give a reason for reopening."
		closesInPastVI = "Thời điểm đóng mới phải ở phía trước."
		closesInPastEN = "The new closing time must be in the future."
		noTargets      = "Chọn ít nhất một lớp hoặc một học viên."
	)
	for _, c := range []struct {
		name      string
		serve     refusing
		status    int
		code      openapi.ErrorCode
		vi        string
		en        string
		detailsVI map[string]interface{}
		detailsEN map[string]interface{}
	}{
		{
			name:   "changing the version of an assignment a student has started",
			serve:  updating(domain.ErrVersionLocked),
			status: http.StatusConflict,
			code:   openapi.VERSIONLOCKED,
			vi:     "Đã có học viên làm bài, không thể đổi phiên bản đề.",
			en:     "A student has already started, so the test version cannot be changed.",
		},
		{
			name:   "updating an assignment that is not there",
			serve:  updating(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy bài giao.",
			en:     "The assignment was not found.",
		},
		{
			name:   "assigning a version that is not published",
			serve:  creating(domain.ErrTestNotPublished),
			status: http.StatusConflict,
			code:   openapi.TESTNOTPUBLISHED,
			vi:     "Chỉ có thể giao một phiên bản đề đã xuất bản.",
			en:     "Only a published version of a test can be assigned.",
		},
		{
			name:      "reopening without a reason",
			serve:     reopening(domain.ErrBlankReason),
			status:    http.StatusBadRequest,
			code:      openapi.VALIDATIONFAILED,
			vi:        blankReasonVI,
			en:        blankReasonEN,
			detailsVI: map[string]interface{}{"reason": blankReasonVI},
			detailsEN: map[string]interface{}{"reason": blankReasonEN},
		},
		{
			name:      "reopening until a time that has passed",
			serve:     reopening(domain.ErrClosesInPast),
			status:    http.StatusBadRequest,
			code:      openapi.VALIDATIONFAILED,
			vi:        closesInPastVI,
			en:        closesInPastEN,
			detailsVI: map[string]interface{}{"closesAt": closesInPastVI},
			detailsEN: map[string]interface{}{"closesAt": closesInPastEN},
		},
		{
			name:   "reopening an assignment that is not closed",
			serve:  reopening(domain.ErrNotClosed),
			status: http.StatusConflict,
			code:   openapi.ASSIGNMENTNOTCLOSED,
			vi:     "Bài giao chưa đóng nên không có gì để mở lại.",
			en:     "The assignment is not closed, so there is nothing to reopen.",
		},
		{
			name:   "deleting an assignment that history refers to",
			serve:  deleting(domain.ErrReferenced),
			status: http.StatusConflict,
			code:   openapi.RESOURCEREFERENCED,
			vi:     "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.",
			en:     "This cannot be deleted because assignments, attempts or history still refer to it.",
		},
		{
			name:      "creating an assignment the domain refuses",
			serve:     creating(&domain.ValidationError{Fields: []domain.FieldError{{Field: "targets", Message: noTargets}}}),
			status:    http.StatusBadRequest,
			code:      openapi.VALIDATIONFAILED,
			vi:        "Dữ liệu bài giao không hợp lệ.",
			en:        "The assignment data is not valid.",
			detailsVI: map[string]interface{}{"targets": noTargets},
			detailsEN: map[string]interface{}{"targets": noTargets},
		},
		{
			name:   "a student opening an assignment that is not theirs",
			serve:  openingAsStudent(domain.ErrForbidden),
			status: http.StatusForbidden,
			code:   openapi.FORBIDDEN,
			vi:     "Bạn không có quyền xem bài này.",
			en:     "You do not have permission to view this assignment.",
		},
	} {
		for _, language := range []struct {
			accept  string
			message string
			details map[string]interface{}
		}{{"", c.vi, c.detailsVI}, {"en", c.en, c.detailsEN}} {
			response := refusedIn(t, language.accept, c.serve)
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
