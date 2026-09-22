package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func (h Tests) DeleteTest(ctx context.Context, request openapi.DeleteTestRequestObject) (openapi.DeleteTestResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{Request: req})
	switch {
	case err == nil:
		return openapi.DeleteTest204Response{}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, "Không tìm thấy dữ liệu."))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteTest409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.")), nil
	case errors.Is(err, domain.ErrReferenced):
		return openapi.DeleteTest409JSONResponse(httpapi.Error(ctx, openapi.RESOURCEREFERENCED, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.")), nil
	default:
		return nil, err
	}
}
