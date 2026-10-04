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
		return openapi.DeleteTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy dữ liệu.", "The data was not found.")))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteTest409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED,
			httpx.Text(ctx, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.",
				"Archive, disable or close this item before deleting it permanently."))), nil
	case errors.Is(err, domain.ErrReferenced):
		return openapi.DeleteTest409JSONResponse(httpapi.Error(ctx, openapi.RESOURCEREFERENCED,
			httpx.Text(ctx, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.",
				"This cannot be deleted because assignments, attempts or history still refer to it."))), nil
	default:
		return nil, err
	}
}
