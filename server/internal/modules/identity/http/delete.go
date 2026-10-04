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

// DeleteUser implements DELETE /admin/users/{id}. In R2 its target is still
// only a disabled student account (T-R2.13 keeps it strict).
func (h Identity) DeleteUser(ctx context.Context, request openapi.DeleteUserRequestObject) (openapi.DeleteUserResponseObject, error) {
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
		return openapi.DeleteUser204Response{}, nil
	case errors.Is(err, domain.ErrStudentNotFound):
		return openapi.DeleteUser404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy dữ liệu.", "The data was not found.")))}, nil
	case errors.Is(err, domain.ErrNotArchived):
		return openapi.DeleteUser409JSONResponse(httpapi.Error(ctx, openapi.RESOURCENOTARCHIVED,
			httpx.Text(ctx, "Cần lưu trữ, vô hiệu hoá hoặc đóng mục này trước khi xoá vĩnh viễn.",
				"Archive, disable or close this item before deleting it permanently."))), nil
	case errors.Is(err, domain.ErrForbidden):
		return openapi.DeleteUser403JSONResponse(httpapi.Error(ctx, openapi.FORBIDDEN, msgStudentForbidden(ctx))), nil
	case errors.Is(err, domain.ErrReferenced):
		by := domain.ReferencedByOther
		var refused *domain.ReferencedError
		if errors.As(err, &refused) {
			by = refused.By
		}
		return openapi.DeleteUser409JSONResponse(httpapi.ErrorWithDetails(ctx, openapi.RESOURCEREFERENCED,
			httpx.Text(ctx, "Không thể xoá vì dữ liệu vẫn được bài giao, bài làm hoặc lịch sử tham chiếu.",
				"This cannot be deleted because assignments, attempts or history still refer to it."),
			map[string]interface{}{"referencedBy": openapi.ReferencedBy(by)})), nil
	default:
		return nil, err
	}
}
