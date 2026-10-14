package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"
)

type failure struct {
	status int
	body   openapi.ErrorResponse
}

func (f *failure) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(f.status)
	return json.NewEncoder(w).Encode(f.body)
}

func (f *failure) VisitCreateRoleResponse(w http.ResponseWriter) error         { return f.write(w) }
func (f *failure) VisitUpdateRoleResponse(w http.ResponseWriter) error         { return f.write(w) }
func (f *failure) VisitSetRolePermissionsResponse(w http.ResponseWriter) error { return f.write(w) }

type translation struct {
	err     error
	status  int
	code    openapi.ErrorCode
	message string
}

func translations(ctx context.Context) []translation {
	return []translation{
		{domain.ErrRoleNotFound, http.StatusNotFound, openapi.NOTFOUND, httpx.Text(ctx, "Không tìm thấy vai trò này.", "No such role.")},
		{domain.ErrNameTaken, http.StatusConflict, openapi.ROLENAMETAKEN, httpx.Text(ctx, "Vai trò này đã tồn tại.", "A role with this name already exists.")},
		{domain.ErrStaleWrite, http.StatusConflict, openapi.STALEWRITE, httpx.Text(ctx, "Vai trò đã được thay đổi ở nơi khác. Hãy tải lại rồi thử lại.", "The role was changed elsewhere. Reload and try again.")},
		{domain.ErrNotAssignable, http.StatusUnprocessableEntity, openapi.ROLENOTASSIGNABLE, httpx.Text(ctx, "Không thể chọn vai trò này.", "This role cannot be chosen.")},
		{domain.ErrRoleLocked, http.StatusUnprocessableEntity, openapi.ROLELOCKED, httpx.Text(ctx, "Quản trị viên luôn có quyền này.", "Admins always have this permission.")},
		{domain.ErrStudentNeedsTakeTests, http.StatusUnprocessableEntity, openapi.STUDENTNEEDSTAKETESTS, httpx.Text(ctx, "Học viên luôn cần quyền làm bài.", "Students always need permission to take tests.")},
		{domain.ErrKeyNotGrantable, http.StatusUnprocessableEntity, openapi.KEYNOTGRANTABLE, httpx.Text(ctx, "Không thể cấp quyền này cho vai trò.", "This permission cannot be granted to a role.")},
		{domain.ErrCannotActOnSelf, http.StatusUnprocessableEntity, openapi.CANNOTACTONSELF, httpx.Text(ctx, "Bạn không thể đổi quyền của chính vai trò của mình.", "You cannot change the permissions of your own role.")},
		{domain.ErrTargetRoleNotAllowed, http.StatusForbidden, openapi.TARGETROLENOTALLOWED, httpx.Text(ctx, "Bạn chỉ đổi được những quyền bạn có.", "You can only change what you can do yourself.")},
	}
}

func failureOf(ctx context.Context, err error) *failure {
	var invalid *validation.Error
	if errors.As(err, &invalid) {
		return &failure{status: http.StatusBadRequest, body: httpapi.Invalid(ctx, httpx.Text(ctx, "Dữ liệu không hợp lệ.", "The data is not valid."), invalid)}
	}
	for _, t := range translations(ctx) {
		if errors.Is(err, t.err) {
			return &failure{status: t.status, body: httpapi.Error(ctx, t.code, t.message)}
		}
	}
	return nil
}
