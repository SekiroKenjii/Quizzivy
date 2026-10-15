package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// CreateUser implements POST /admin/users.
func (h Identity) CreateUser(ctx context.Context, request openapi.CreateUserRequestObject) (openapi.CreateUserResponseObject, error) {
	if h.app == nil || h.app.Commands.CreateUser == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.NewUser{
		Email:    string(request.Body.Email),
		FullName: request.Body.FullName,
		RoleID:   request.Body.RoleId.String(),
		Delivery: domain.Delivery(request.Body.Delivery),
	}
	if request.Body.ClassId != nil {
		in.ClassID = request.Body.ClassId.String()
	}

	result, err := h.app.Commands.CreateUser.Handle(ctx, command.CreateUser{Request: req, Input: in})
	if err != nil {
		return createUserFailure(ctx, err)
	}
	out := openapi.CreateUser201JSONResponse{Headers: openapi.CreateUser201ResponseHeaders{CacheControl: httpapi.Ptr("no-store")}}
	out.Body.User = toAPIAdminUser(result.User)
	if result.TemporaryPassword != "" {
		out.Body.TemporaryPassword = httpapi.Ptr(result.TemporaryPassword)
	}
	return out, nil
}

func createUserFailure(ctx context.Context, err error) (openapi.CreateUserResponseObject, error) {
	var invalid *validation.Error
	switch {
	case errors.As(err, &invalid):
		return openapi.CreateUser400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.Invalid(ctx, httpx.Text(ctx, "Dữ liệu tài khoản không hợp lệ.", "The account data is not valid."), invalid))}, nil
	case errors.Is(err, domain.ErrClassNotAllowed):
		return openapi.CreateUser400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, "classId",
			httpx.Text(ctx, "Chỉ thêm được vào lớp khi vai trò là học viên và bạn có quyền trên mọi lớp.",
				"A class can be set only for a student role, and only by someone who reaches every class.")))}, nil
	case errors.Is(err, accessdomain.ErrRoleNotFound), errors.Is(err, domain.ErrClassNotFound):
		return openapi.CreateUser404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy vai trò hoặc lớp học.", "The role or the class was not found.")))}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.CreateUser403JSONResponse(httpapi.Error(ctx, openapi.TARGETROLENOTALLOWED,
			httpx.Text(ctx, "Bạn chỉ giao được những quyền bạn có.", "You can only give what you can do yourself."))), nil
	case errors.Is(err, domain.ErrEmailTaken):
		return openapi.CreateUser409JSONResponse(httpapi.Error(ctx, openapi.EMAILTAKEN,
			httpx.Text(ctx, "Địa chỉ email này đã được dùng cho một tài khoản khác.",
				"This email address is already used by another account."))), nil
	case errors.Is(err, accessdomain.ErrNotAssignable):
		return openapi.CreateUser422JSONResponse(httpapi.Error(ctx, openapi.ROLENOTASSIGNABLE,
			httpx.Text(ctx, "Không thể chọn vai trò này.", "This role cannot be chosen."))), nil
	case errors.Is(err, domain.ErrPasswordSignInDisabled):
		return openapi.CreateUser422JSONResponse(passwordSignInOff(ctx)), nil
	case errors.Is(err, domain.ErrSetPasswordUnavailable):
		return nil, httpx.ErrNotImplemented
	default:
		return nil, err
	}
}

func toAPIAdminUser(u domain.AdminUser) openapi.AdminUserRow {
	role := openapi.AdminUserRole{
		Id:    httpapi.ParseUUID(u.Role.ID),
		Name:  u.Role.Name,
		Icon:  openapi.RoleIcon(u.Role.Icon),
		Color: openapi.RoleColor(u.Role.Color),
	}
	if u.Role.BuiltinKey != "" {
		role.BuiltinKey = httpapi.Ptr(openapi.BuiltinRoleKey(u.Role.BuiltinKey))
	}
	row := openapi.AdminUserRow{
		Id:           httpapi.ParseUUID(u.ID),
		FullName:     u.FullName,
		Email:        openapi_types.Email(u.Email),
		Role:         role,
		LastActiveAt: u.LastActiveAt,
		Status:       openapi.AccountStatus(u.Status),
		CreatedAt:    u.CreatedAt,
	}
	if u.AvatarURL != "" {
		row.AvatarUrl = httpapi.Ptr(u.AvatarURL)
	}
	row.SignIn.Password, row.SignIn.Google = u.HasPassword, u.HasGoogle
	row.Classes.Count, row.Classes.FirstName = u.ClassCount, u.FirstClass
	return row
}
