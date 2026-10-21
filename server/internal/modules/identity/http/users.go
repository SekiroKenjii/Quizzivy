package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ListUsers implements GET /admin/users.
func (h Identity) ListUsers(ctx context.Context, request openapi.ListUsersRequestObject) (openapi.ListUsersResponseObject, error) {
	if h.app == nil || h.app.Queries.ListUsers == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.AdminUserQuery{Tab: domain.TabAll}
	params := request.Params
	if params.Tab != nil {
		in.Tab = domain.AdminTab(*params.Tab)
	}
	if params.Status != nil {
		in.Status = domain.AccountStatus(*params.Status)
	}
	if params.Q != nil {
		in.Query = string(*params.Q)
	}
	if params.Email != nil {
		in.Email = string(*params.Email)
	}
	if params.Page != nil {
		in.Page = int(*params.Page)
	}
	if params.Limit != nil {
		in.Limit = int(*params.Limit)
	}

	result, err := h.app.Queries.ListUsers.Handle(ctx, query.ListUsers{Query: in})
	if err != nil {
		return nil, err
	}
	out := openapi.ListUsers200JSONResponse{
		Items: make([]openapi.AdminUserRow, len(result.Items)),
		Facets: openapi.AdminUserFacets{
			All:      result.Facets.All,
			Teachers: result.Facets.Teachers,
			Students: result.Facets.Students,
			Admins:   result.Facets.Admins,
			Disabled: result.Facets.Disabled,
		},
		Page:     result.Page.Number,
		PageSize: result.Page.Size,
		Total:    result.Page.Total,
	}
	for i, user := range result.Items {
		out.Items[i] = toAPIAdminUser(user)
	}
	return out, nil
}

// GetUser implements GET /admin/users/{id}.
func (h Identity) GetUser(ctx context.Context, request openapi.GetUserRequestObject) (openapi.GetUserResponseObject, error) {
	if h.app == nil || h.app.Queries.GetUser == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	result, err := h.app.Queries.GetUser.Handle(ctx, query.GetUser{Request: req, ID: request.Id.String()})
	if errors.Is(err, domain.ErrUserNotFound) {
		return openapi.GetUser404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy người dùng.", "The user was not found.")))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetUser200JSONResponse(toAPIAdminUserDetail(result)), nil
}

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
	if body, ok := overLimit(ctx, err); ok {
		return openapi.CreateUser409JSONResponse(body), nil
	}
	var invalid *validation.Error
	switch {
	case errors.As(err, &invalid):
		return openapi.CreateUser400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.Invalid(ctx, httpx.Text(ctx, "Dữ liệu tài khoản không hợp lệ.", "The account data is not valid."), invalid))}, nil
	case errors.Is(err, domain.ErrClassNotAllowed):
		return openapi.CreateUser400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, "classId",
			httpx.Text(ctx, "Chỉ thêm được vào lớp khi vai trò là học viên và bạn có quyền trên mọi lớp.",
				"A class can be set only for a student role, and only by someone who reaches every class.")))}, nil
	case errors.Is(err, accessdomain.ErrRoleNotFound):
		return openapi.CreateUser404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy vai trò này.", "No such role.")))}, nil
	case errors.Is(err, domain.ErrClassNotFound):
		return openapi.CreateUser404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy lớp học này.", "No such class.")))}, nil
	case errors.Is(err, domain.ErrRoleChanged):
		return openapi.CreateUser409JSONResponse(httpapi.Error(ctx, openapi.STALEWRITE,
			httpx.Text(ctx, "Vai trò vừa được thay đổi. Hãy thử lại.", "The role was just changed. Try again."))), nil
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

func toAPIAdminUserDetail(result query.GetUserResult) openapi.AdminUserDetail {
	d := result.Detail
	out := openapi.AdminUserDetail{
		User:               toAPIAdminUser(d.User),
		Classes:            make([]openapi.AdminUserClass, len(d.Classes)),
		InProgressAttempts: d.InProgressAttempts,
		Activity:           make([]openapi.AdminUserActivity, len(d.Events)),
		Can: openapi.AdminUserCan{
			ChangeRole:    result.Can.ChangeRole,
			ResetPassword: result.Can.ResetPassword,
			Disable:       result.Can.Disable,
			Enable:        result.Can.Enable,
			SignOut:       result.Can.SignOut,
			Transfer:      result.Can.Transfer,
		},
	}
	out.OwnedContent.Classes, out.OwnedContent.Tests, out.OwnedContent.Questions = d.Owned.Classes, d.Owned.Tests, d.Owned.Questions
	out.OwnedContent.Media, out.OwnedContent.Assignments = d.Owned.Media, d.Owned.Assignments
	if d.CreatedBy != nil {
		out.CreatedBy = &struct {
			FullName string       `json:"fullName"`
			Id       openapi.Uuid `json:"id"`
		}{FullName: d.CreatedBy.FullName, Id: httpapi.ParseUUID(d.CreatedBy.ID)}
	}
	for i, c := range d.Classes {
		out.Classes[i] = openapi.AdminUserClass{Id: httpapi.ParseUUID(c.ID), Name: c.Name}
	}
	for i, e := range d.Events {
		out.Activity[i] = openapi.AdminUserActivity{Kind: openapi.AdminUserActivityKind(e.Kind), At: e.At, Detail: e.Detail}
	}
	return out
}
