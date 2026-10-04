package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"quizzivy/gen/openapi"
	mediadomain "quizzivy/internal/modules/media/domain"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"strconv"
)

type groupResponse struct {
	status  int
	group   *openapi.StoredQuestionGroup
	list    *openapi.ListQuestionGroups200JSONResponse
	problem *openapi.ErrorResponse
}

func (h Tests) groupReply(ctx context.Context, stored domain.StoredGroup, err error, status int) (*groupResponse, error) {
	if err != nil {
		return groupFailure(ctx, err)
	}
	group, err := h.writeGroup(ctx, stored)
	if err != nil {
		return nil, err
	}
	return &groupResponse{status: status, group: &group}, nil
}

func groupListReply(result query.GroupsResult) (*groupResponse, error) {
	out := openapi.ListQuestionGroups200JSONResponse{Page: result.Page.Number, PageSize: result.Page.Size, Total: result.Page.Total, Items: make([]openapi.QuestionGroupSummary, len(result.Items))}
	for i, item := range result.Items {
		points, err := strconv.ParseFloat(item.TotalPoints, 64)
		if err != nil {
			return nil, err
		}
		out.Items[i] = openapi.QuestionGroupSummary{Id: httpapi.ParseUUID(item.ID), Title: item.Title, Revision: item.Revision, QuestionCount: item.QuestionCount, RecordingCount: item.RecordingCount, TotalPoints: points, Tags: item.Tags, ArchivedAt: item.ArchivedAt, UpdatedAt: item.UpdatedAt}
	}
	return &groupResponse{status: 200, list: &out}, nil
}

func groupFailure(ctx context.Context, err error) (*groupResponse, error) {
	status := http.StatusConflict
	code := openapi.GROUPCONFLICT
	message := httpx.Text(ctx, "Nhóm hoặc một định danh đã thay đổi. Vui lòng tải lại trước khi lưu.",
		"The group or one of its ids has changed. Please reload before saving.")
	var invalid *domain.GroupError
	switch {
	case errors.Is(err, domain.ErrGroupUnavailable):
		return nil, httpx.ErrNotImplemented
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = openapi.NOTFOUND
		message = httpx.Text(ctx, "Không tìm thấy nhóm câu hỏi.", "The question group was not found.")
	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
		code = openapi.FORBIDDEN
		message = httpx.Text(ctx, "Bạn không có quyền sửa loại nhóm câu hỏi này.",
			"You do not have permission to edit this kind of question group.")
	case errors.Is(err, domain.ErrStaleWrite):
		code = openapi.STALEWRITE
		message = httpx.Text(ctx, "Nhóm hoặc đề đã được sửa ở nơi khác. Vui lòng tải lại trước khi lưu.",
			"The group or the test was edited elsewhere. Please reload before saving.")
	case errors.Is(err, domain.ErrNotArchived):
		code = openapi.RESOURCENOTARCHIVED
		message = httpx.Text(ctx, "Hãy lưu trữ nhóm trước khi xoá vĩnh viễn.",
			"Archive the group before deleting it permanently.")
	case errors.Is(err, domain.ErrReferenced):
		code = openapi.RESOURCEREFERENCED
		message = httpx.Text(ctx, "Không thể xoá nhóm vì nhóm hoặc câu hỏi trong nhóm vẫn đang được dùng ở nơi khác.",
			"The group cannot be deleted because it, or one of its questions, is still used elsewhere.")
	case errors.Is(err, domain.ErrGroupConflict):
	case errors.Is(err, questionsdomain.ErrMediaNotFound), errors.Is(err, mediadomain.ErrNotFound):
		status = http.StatusUnprocessableEntity
		code = openapi.VALIDATIONFAILED
		message = httpx.Text(ctx, "Một tệp ngữ liệu không tồn tại hoặc đã bị xoá.",
			"A material file does not exist or has been deleted.")
	case errors.As(err, &invalid):
		switch invalid.Rule {
		case "group_owner_archived":
			code = openapi.TESTARCHIVED
			message = httpx.Text(ctx, "Hãy khôi phục đề đã lưu trữ trước khi thay đổi nhóm.",
				"Restore the archived test before changing the group.")
		case "group_archived":
			message = httpx.Text(ctx, "Hãy khôi phục nhóm đã lưu trữ trước khi chỉnh sửa.",
				"Restore the archived group before editing it.")
		case "group_bank_only":
			message = httpx.Text(ctx, "Nhóm thuộc đề theo trạng thái lưu trữ của đề.",
				"A group that belongs to a test follows the test's archive state.")
		default:
			status = http.StatusUnprocessableEntity
			code = openapi.VALIDATIONFAILED
			message = httpx.Text(ctx, "Nội dung hoặc liên kết của nhóm chưa hợp lệ. Hãy kiểm tra câu hỏi và ngữ liệu.",
				"The group's content or links are not valid. Check its questions and materials.")
		}
	default:
		return nil, err
	}
	problem := httpapi.Error(ctx, code, message)
	if invalid != nil {
		problem.Error.Details = &map[string]interface{}{"rule": invalid.Rule, "questionId": invalid.QuestionID, "stimulusId": invalid.StimulusID, "gapId": invalid.GapID}
	}
	return &groupResponse{status: status, problem: &problem}, nil
}

func (r *groupResponse) write(w http.ResponseWriter) error {
	if r.status == http.StatusNoContent {
		w.WriteHeader(r.status)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	if r.problem != nil {
		return json.NewEncoder(w).Encode(r.problem)
	}
	if r.list != nil {
		return json.NewEncoder(w).Encode(r.list)
	}
	return json.NewEncoder(w).Encode(r.group)
}

func (r *groupResponse) VisitGetQuestionGroupResponse(w http.ResponseWriter) error { return r.write(w) }

func (r *groupResponse) VisitCreateQuestionGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r *groupResponse) VisitUpdateQuestionGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r *groupResponse) VisitCopyQuestionGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r *groupResponse) VisitArchiveQuestionGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r *groupResponse) VisitDeleteQuestionGroupResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r *groupResponse) VisitListQuestionGroupsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
