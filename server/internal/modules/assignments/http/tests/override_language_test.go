package http_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/domain"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	"quizzivy/internal/shared/cqrs"
)

func extending(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		Extend: cqrs.HandlerFunc[command.Extend, domain.Assignment](func(context.Context, command.Extend) (domain.Assignment, error) {
			return domain.Assignment{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.ExtendAssignment(ctx, openapi.ExtendAssignmentRequestObject{Id: uuid.New(), Body: &openapi.ExtendAssignmentJSONRequestBody{Minutes: 30}})
		if err != nil {
			return err
		}
		return response.VisitExtendAssignmentResponse(w)
	}
}

func overriding(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		SetOverrides: cqrs.HandlerFunc[command.SetOverrides, []domain.StudentOverride](func(context.Context, command.SetOverrides) ([]domain.StudentOverride, error) {
			return nil, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.SetStudentOverrides(ctx, openapi.SetStudentOverridesRequestObject{
			Id: uuid.New(), Body: &openapi.SetStudentOverridesJSONRequestBody{StudentIds: []openapi.Uuid{uuid.New()}, Reason: "Ốm"},
		})
		if err != nil {
			return err
		}
		return response.VisitSetStudentOverridesResponse(w)
	}
}

func removingAnOverride(outcome error) refusing {
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		DeleteOverride: cqrs.HandlerFunc[command.DeleteOverride, cqrs.Nothing](func(context.Context, command.DeleteOverride) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, outcome
		}),
	}})
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteStudentOverride(ctx, openapi.DeleteStudentOverrideRequestObject{Id: uuid.New(), StudentId: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitDeleteStudentOverrideResponse(w)
	}
}

func TestExtensionAndOverrideRefusalsSpeakTheCallersLanguage(t *testing.T) {
	missing, other := uuid.NewString(), uuid.NewString()
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
			name:   "extending a closed assignment",
			serve:  extending(domain.ErrClosed),
			status: http.StatusConflict,
			code:   openapi.ASSIGNMENTCLOSED,
			vi:     "Bài giao đã đóng nên không thể gia hạn. Hãy mở lại bài giao.",
			en:     "The assignment has closed, so it cannot be extended. Reopen it instead.",
		},
		{
			name:   "extending an assignment that is not there",
			serve:  extending(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy bài giao.",
			en:     "The assignment was not found.",
		},
		{
			name:   "overriding on an assignment that is not there",
			serve:  overriding(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy bài giao.",
			en:     "The assignment was not found.",
		},
		{
			name:   "extending a student whose close has passed",
			serve:  overriding(domain.ErrClosed),
			status: http.StatusConflict,
			code:   openapi.ASSIGNMENTCLOSED,
			vi:     "Có học viên đã hết thời hạn nên không thể gia hạn thêm. Hãy đặt thời điểm đóng mới.",
			en:     "A student's close has passed, so it cannot be extended. Set a new closing time.",
		},
		{
			name:      "overriding students who are not on the assignment",
			serve:     overriding(&domain.NotTargetedError{StudentIDs: []string{missing, other}}),
			status:    http.StatusUnprocessableEntity,
			code:      openapi.VALIDATIONFAILED,
			vi:        "Có học viên không thuộc bài giao này.",
			en:        "Some students are not on this assignment.",
			detailsVI: map[string]interface{}{"studentIds": "Không thuộc bài giao này: " + missing + ", " + other},
			detailsEN: map[string]interface{}{"studentIds": "Not on this assignment: " + missing + ", " + other},
		},
		{
			name:      "an override request the domain refuses",
			serve:     overriding(&domain.ValidationError{Fields: []domain.FieldError{{Field: "reason", Message: "Hãy ghi lý do."}}}),
			status:    http.StatusBadRequest,
			code:      openapi.VALIDATIONFAILED,
			vi:        "Dữ liệu bài giao không hợp lệ.",
			en:        "The assignment data is not valid.",
			detailsVI: map[string]interface{}{"reason": "Hãy ghi lý do."},
			detailsEN: map[string]interface{}{"reason": "Hãy ghi lý do."},
		},
		{
			name:   "removing an override that is not there",
			serve:  removingAnOverride(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy ngoại lệ của học viên này.",
			en:     "The student's override was not found.",
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

func TestAnOverrideRequestReachesTheCommandAsTheTeacherAskedIt(t *testing.T) {
	student, until := uuid.New(), time.Now().Add(time.Hour).Truncate(time.Second)
	minutes, attempts, notify := 90, 2, true
	var got command.SetOverrides
	transport := assignmentshttp.NewAssignments(&application.Application{Commands: application.Commands{
		SetOverrides: cqrs.HandlerFunc[command.SetOverrides, []domain.StudentOverride](func(_ context.Context, cmd command.SetOverrides) ([]domain.StudentOverride, error) {
			got = cmd
			return []domain.StudentOverride{{StudentID: student.String(), StudentName: "Lan", ClosesAt: &until, DurationMin: &minutes, ExtraAttempts: attempts, Reason: "Ốm"}}, nil
		}),
	}})
	var answered openapi.SetStudentOverridesResponseObject
	response := refusedIn(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.SetStudentOverrides(ctx, openapi.SetStudentOverridesRequestObject{Id: uuid.New(), Body: &openapi.SetStudentOverridesJSONRequestBody{
			StudentIds: []openapi.Uuid{student}, ClosesAt: &until, DurationMinutes: &minutes, ExtraAttempts: &attempts, Reason: "Ốm", Notify: &notify,
		}})
		answered = out
		if err != nil {
			return err
		}
		return out.VisitSetStudentOverridesResponse(w)
	})
	if response.Code != http.StatusOK {
		t.Fatalf("answered %d: %s", response.Code, response.Body.String())
	}
	in := got.Input
	if len(in.StudentIDs) != 1 || in.StudentIDs[0] != student.String() || in.ClosesAt == nil || !in.ClosesAt.Equal(until) ||
		in.DurationMin == nil || *in.DurationMin != 90 || in.ExtraAttempts == nil || *in.ExtraAttempts != 2 || !in.Notify || in.Reason != "Ốm" || in.ExtendBy != nil {
		t.Errorf("the command carries %+v", in)
	}
	if ok, isOK := answered.(openapi.SetStudentOverrides200JSONResponse); !isOK || len(ok.Items) != 1 || ok.Items[0].StudentName != "Lan" || ok.Items[0].ExtraAttempts != 2 {
		t.Errorf("answered %+v", answered)
	}
}
