package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

type refusal struct {
	status     int
	code       openapi.ErrorCode
	message    string
	details    map[string]interface{}
	violations []string
}

type refusing func(ctx context.Context, w http.ResponseWriter) error

func refusalIn(t *testing.T, acceptLanguage string, serve refusing) refusal {
	t.Helper()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: uuid.NewString()}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/teacher/tests", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body openapi.PublishConflict
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v: %s", err, response.Body.String())
	}
	out := refusal{status: response.Code, code: body.Error.Code, message: body.Error.Message}
	if body.Error.Details != nil {
		out.details = *body.Error.Details
	}
	if body.Violations != nil {
		for _, violation := range *body.Violations {
			out.violations = append(out.violations, violation.Message)
		}
	}
	return out
}

func deletingTest(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		Delete: cqrs.HandlerFunc[command.Delete, cqrs.Nothing](func(context.Context, command.Delete) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteTest(ctx, openapi.DeleteTestRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitDeleteTestResponse(w)
	}
}

func updatingTest(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		Update: cqrs.HandlerFunc[command.Update, domain.Test](func(context.Context, command.Update) (domain.Test, error) {
			return domain.Test{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.UpdateTest(ctx, openapi.UpdateTestRequestObject{Id: uuid.New(), Body: &openapi.UpdateTestJSONRequestBody{}})
		if err != nil {
			return err
		}
		return response.VisitUpdateTestResponse(w)
	}
}

func publishingTest(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		Publish: cqrs.HandlerFunc[command.Publish, domain.Version](func(context.Context, command.Publish) (domain.Version, error) {
			return domain.Version{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.PublishTest(ctx, openapi.PublishTestRequestObject{Id: uuid.New()})
		if err != nil {
			return err
		}
		return response.VisitPublishTestResponse(w)
	}
}

func deletingGroup(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		DeleteGroup: cqrs.HandlerFunc[command.DeleteGroup, cqrs.Nothing](func(context.Context, command.DeleteGroup) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteQuestionGroup(ctx, openapi.DeleteQuestionGroupRequestObject{Id: uuid.New(), Params: openapi.DeleteQuestionGroupParams{ExpectedRevision: 1}})
		if err != nil {
			return err
		}
		return response.VisitDeleteQuestionGroupResponse(w)
	}
}

func updatingGroup(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		UpdateGroup: cqrs.HandlerFunc[command.UpdateGroup, domain.StoredGroup](func(context.Context, command.UpdateGroup) (domain.StoredGroup, error) {
			return domain.StoredGroup{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		id := uuid.New()
		var body openapi.UpdateQuestionGroupJSONRequestBody
		input := `{"bundle":{"group":{"id":"` + id.String() + `","title":"Nhóm","members":[],"stimuli":[],"recordings":[]},"questions":[]},"expectedRevision":1}`
		if err := json.Unmarshal([]byte(input), &body); err != nil {
			return err
		}
		response, err := transport.UpdateQuestionGroup(ctx, openapi.UpdateQuestionGroupRequestObject{Id: id, Body: &body})
		if err != nil {
			return err
		}
		return response.VisitUpdateQuestionGroupResponse(w)
	}
}

func restoringVersion(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		CreateDraftFromVersion: cqrs.HandlerFunc[command.CreateDraftFromVersion, domain.Test](func(context.Context, command.CreateDraftFromVersion) (domain.Test, error) {
			return domain.Test{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.CreateDraftFromTestVersion(ctx, openapi.CreateDraftFromTestVersionRequestObject{Id: uuid.New(), Version: 1, Body: &openapi.CreateDraftFromTestVersionJSONRequestBody{}})
		if err != nil {
			return err
		}
		return response.VisitCreateDraftFromTestVersionResponse(w)
	}
}

func deletingVersion(failure error) refusing {
	transport := testshttp.NewTests(&application.Application{Commands: application.Commands{
		DeleteVersion: cqrs.HandlerFunc[command.DeleteVersion, cqrs.Nothing](func(context.Context, command.DeleteVersion) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, failure
		}),
	}}, nil)
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.DeleteTestVersion(ctx, openapi.DeleteTestVersionRequestObject{Id: uuid.New(), Version: 1})
		if err != nil {
			return err
		}
		return response.VisitDeleteTestVersionResponse(w)
	}
}

func TestTestRefusalsSpeakTheCallersLanguage(t *testing.T) {
	const (
		notPublishableVI = "Đề chưa thể xuất bản. Vui lòng sửa các vấn đề được đánh dấu."
		notPublishableEN = "The test cannot be published yet. Please fix the marked problems."
		wordedByDomain   = "Tổng điểm của đề không được vượt quá 999999,99."
	)
	for _, c := range []struct {
		name        string
		serve       refusing
		status      int
		code        openapi.ErrorCode
		vi          string
		en          string
		detail      string
		detailVI    string
		detailEN    string
		violationVI string
		violationEN string
	}{
		{
			name:   "deleting a test that is still referred to",
			serve:  deletingTest(domain.ErrReferenced),
			status: http.StatusConflict,
			code:   openapi.RESOURCEREFERENCED,
			vi:     "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.",
			en:     "This cannot be deleted because assignments, attempts or history still refer to it.",
		},
		{
			name:   "deleting a test that is not archived",
			serve:  deletingTest(domain.ErrNotArchived),
			status: http.StatusConflict,
			code:   openapi.RESOURCENOTARCHIVED,
			vi:     "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.",
			en:     "Archive, disable or close this item before deleting it permanently.",
		},
		{
			name:   "saving a test edited elsewhere",
			serve:  updatingTest(domain.ErrStaleWrite),
			status: http.StatusConflict,
			code:   openapi.STALEWRITE,
			vi:     "Đề đã được sửa ở nơi khác. Vui lòng tải lại trước khi lưu.",
			en:     "The test was edited elsewhere. Please reload before saving.",
		},
		{
			name:   "saving a test that is not there",
			serve:  updatingTest(domain.ErrNotFound),
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy đề.",
			en:     "The test was not found.",
		},
		{
			name:     "saving a test that names a deleted question",
			serve:    updatingTest(domain.ErrUnknownQuestion),
			status:   http.StatusBadRequest,
			code:     openapi.VALIDATIONFAILED,
			vi:       "Đề tham chiếu câu hỏi không tồn tại.",
			en:       "The test refers to a question that does not exist.",
			detail:   "sections",
			detailVI: "Một câu hỏi trong đề đã bị xoá.",
			detailEN: "A question in the test has been deleted.",
		},
		{
			name: "publishing a draft the domain found a problem in",
			serve: publishingTest(&domain.PublishValidationError{Violations: []domain.Violation{
				{Rule: domain.TotalPointsValid, Message: wordedByDomain},
			}}),
			status:      http.StatusConflict,
			code:        openapi.PUBLISHVALIDATIONFAILED,
			vi:          notPublishableVI,
			en:          notPublishableEN,
			violationVI: wordedByDomain,
			violationEN: wordedByDomain,
		},
		{
			name:        "publishing a draft with no sections",
			serve:       publishingTest(domain.ErrNoContent),
			status:      http.StatusConflict,
			code:        openapi.PUBLISHVALIDATIONFAILED,
			vi:          notPublishableVI,
			en:          notPublishableEN,
			violationVI: "Đề chưa có phần nào để xuất bản.",
			violationEN: "The test has no sections to publish.",
		},
		{
			name:   "deleting a group that is still used",
			serve:  deletingGroup(domain.ErrReferenced),
			status: http.StatusConflict,
			code:   openapi.RESOURCEREFERENCED,
			vi:     "Không thể xoá nhóm vì nhóm hoặc câu hỏi trong nhóm vẫn đang được dùng ở nơi khác.",
			en:     "The group cannot be deleted because it, or one of its questions, is still used elsewhere.",
		},
		{
			name:     "saving a group whose links are not valid",
			serve:    updatingGroup(&domain.GroupError{Rule: "group_reference"}),
			status:   http.StatusUnprocessableEntity,
			code:     openapi.VALIDATIONFAILED,
			vi:       "Nội dung hoặc liên kết của nhóm chưa hợp lệ. Hãy kiểm tra câu hỏi và ngữ liệu.",
			en:       "The group's content or links are not valid. Check its questions and materials.",
			detail:   "rule",
			detailVI: "group_reference",
			detailEN: "group_reference",
		},
		{
			name:   "restoring a version over a draft whose group question is used elsewhere",
			serve:  restoringVersion(domain.ErrDraftReferenced),
			status: http.StatusConflict,
			code:   openapi.RESOURCEREFERENCED,
			vi:     "Không thể thay bản nháp vì một câu hỏi trong nhóm của bản nháp vẫn đang được dùng ở nơi khác.",
			en:     "The draft cannot be replaced because a question in one of its groups is still used elsewhere.",
		},
		{
			name:   "deleting the default version",
			serve:  deletingVersion(domain.ErrCurrentVersion),
			status: http.StatusConflict,
			code:   openapi.VERSIONISCURRENT,
			vi:     "Hãy chọn một phiên bản mặc định khác trước khi xoá phiên bản này.",
			en:     "Choose another default version before deleting this one.",
		},
	} {
		for _, language := range []struct {
			accept    string
			want      string
			detail    string
			violation string
		}{{"", c.vi, c.detailVI, c.violationVI}, {"en", c.en, c.detailEN, c.violationEN}} {
			got := refusalIn(t, language.accept, c.serve)
			if got.status != c.status || got.code != c.code {
				t.Errorf("%s with Accept-Language %q answered %d %s, want %d %s", c.name, language.accept, got.status, got.code, c.status, c.code)
			}
			if got.message != language.want {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, got.message, language.want)
			}
			if c.detail != "" && got.details[c.detail] != language.detail {
				t.Errorf("%s with Accept-Language %q carries details.%s %v, want %q", c.name, language.accept, c.detail, got.details[c.detail], language.detail)
			}
			if said := strings.Join(got.violations, "\n"); said != language.violation {
				t.Errorf("%s with Accept-Language %q lists the violations %q, want %q", c.name, language.accept, said, language.violation)
			}
		}
	}
}
