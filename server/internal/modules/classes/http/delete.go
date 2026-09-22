package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/actor"
)

func (h Classes) DeleteClass(ctx context.Context, request openapi.DeleteClassRequestObject) (openapi.DeleteClassResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{ClassID: request.Id.String(), Actor: actor.Actor{ID: principal.UserID, IP: httpx.RequestMetaFromContext(ctx).IP, UserAgent: httpx.RequestMetaFromContext(ctx).UserAgent}})
	switch {
	case err == nil:
		return openapi.DeleteClass204Response{}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteClass404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, "Không tìm thấy dữ liệu."))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteClass409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.")), nil
	case errors.Is(err, domain.ErrReferenced):
		return openapi.DeleteClass409JSONResponse(httpapi.Error(ctx, openapi.RESOURCEREFERENCED, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.")), nil
	default:
		return nil, err
	}
}
