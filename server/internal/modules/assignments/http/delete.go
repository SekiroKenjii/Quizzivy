package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"
)

func (h Assignments) DeleteAssignment(ctx context.Context, request openapi.DeleteAssignmentRequestObject) (openapi.DeleteAssignmentResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{Request: req, Now: time.Now().UTC()})
	switch {
	case err == nil:
		return openapi.DeleteAssignment204Response{}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, "Không tìm thấy dữ liệu."))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteAssignment409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.")), nil
	case errors.Is(err, domain.ErrReferenced):
		return openapi.DeleteAssignment409JSONResponse(httpapi.Error(ctx, openapi.RESOURCEREFERENCED, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.")), nil
	default:
		return nil, err
	}
}
