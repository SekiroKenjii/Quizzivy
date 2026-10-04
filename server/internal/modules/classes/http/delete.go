package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func (h Classes) DeleteClass(ctx context.Context, request openapi.DeleteClassRequestObject) (openapi.DeleteClassResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{ClassID: request.Id.String(), Actor: who})
	switch {
	case err == nil:
		return openapi.DeleteClass204Response{}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteClass404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy dữ liệu.", "The data was not found.")))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteClass409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED,
			httpx.Text(ctx, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.",
				"Archive, disable or close this item before deleting it permanently."))), nil
	case errors.Is(err, domain.ErrReferenced):
		by := domain.ReferencedByOther
		var refused *domain.ReferencedError
		if errors.As(err, &refused) {
			by = refused.By
		}
		return openapi.DeleteClass409JSONResponse(httpapi.ErrorWithDetails(ctx, openapi.RESOURCEREFERENCED,
			httpx.Text(ctx, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.",
				"This cannot be deleted because assignments, attempts or history still refer to it."),
			map[string]interface{}{"referencedBy": openapi.ReferencedBy(by)})), nil
	default:
		return nil, err
	}
}
