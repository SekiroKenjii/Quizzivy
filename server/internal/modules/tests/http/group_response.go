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
	message := "Nhóm hoặc một định danh đã thay đổi. Vui lòng tải lại trước khi lưu."
	var invalid *domain.GroupError
	switch {
	case errors.Is(err, domain.ErrGroupUnavailable):
		return nil, httpx.ErrNotImplemented
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = openapi.NOTFOUND
		message = "Không tìm thấy nhóm câu hỏi."
	case errors.Is(err, domain.ErrStaleWrite):
		code = openapi.STALEWRITE
		message = "Nhóm hoặc đề đã được sửa ở nơi khác. Vui lòng tải lại trước khi lưu."
	case errors.Is(err, domain.ErrNotArchived):
		code = openapi.RESOURCENOTARCHIVED
		message = "Hãy lưu trữ nhóm trước khi xoá vĩnh viễn."
	case errors.Is(err, domain.ErrGroupConflict):
	case errors.Is(err, questionsdomain.ErrMediaNotFound), errors.Is(err, mediadomain.ErrNotFound):
		status = http.StatusUnprocessableEntity
		code = openapi.VALIDATIONFAILED
		message = "Một tệp ngữ liệu không tồn tại hoặc đã bị xoá."
	case errors.As(err, &invalid):
		switch invalid.Rule {
		case "group_owner_archived":
			code = openapi.TESTARCHIVED
			message = "Hãy khôi phục đề đã lưu trữ trước khi thay đổi nhóm."
		case "group_archived":
			message = "Hãy khôi phục nhóm đã lưu trữ trước khi chỉnh sửa."
		case "group_bank_only":
			message = "Nhóm thuộc đề theo trạng thái lưu trữ của đề."
		default:
			status = http.StatusUnprocessableEntity
			code = openapi.VALIDATIONFAILED
			message = "Nội dung hoặc liên kết của nhóm chưa hợp lệ. Hãy kiểm tra câu hỏi và ngữ liệu."
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
