package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func (h Identity) DeleteStudent(ctx context.Context, request openapi.DeleteStudentRequestObject) (openapi.DeleteStudentResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.DeleteStudent.Handle(ctx, command.DeleteStudent{Request: req, ID: request.Id.String()})
	switch {
	case err == nil:
		return openapi.DeleteStudent204Response{}, nil
	case errors.Is(err, domain.ErrStudentNotFound):
		return openapi.DeleteStudent404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, "Không tìm thấy dữ liệu."))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteStudent409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.")), nil
	case errors.Is(err, domain.ErrReferenced):
		return openapi.DeleteStudent409JSONResponse(httpapi.Error(ctx, openapi.RESOURCEREFERENCED, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.")), nil
	default:
		return nil, err
	}
}
