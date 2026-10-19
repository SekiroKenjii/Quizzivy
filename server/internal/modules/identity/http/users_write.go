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

func userIDs(ids []openapi.Uuid) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func userNotFound(ctx context.Context) openapi.NotFoundJSONResponse {
	return openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy người dùng.", "The user was not found.")))
}

func targetNotAllowed(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.TARGETROLENOTALLOWED,
		httpx.Text(ctx, "Bạn chỉ thao tác được trên tài khoản có quyền nằm trong quyền của bạn.",
			"You can only act on accounts whose permissions are within your own."))
}

func cannotActOnSelf(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.CANNOTACTONSELF,
		httpx.Text(ctx, "Bạn không thể thực hiện thao tác này trên chính tài khoản của mình.",
			"You cannot do this to your own account."))
}

func lastAdmin(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.LASTADMIN,
		httpx.Text(ctx, "Đây là quản trị viên cuối cùng còn hoạt động. Hãy thêm một quản trị viên khác trước.",
			"This is the last active Admin. Add another Admin first."))
}

func accountDisabled(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.ACCOUNTDISABLED,
		httpx.Text(ctx, "Tài khoản này đang bị vô hiệu hoá. Hãy bật lại tài khoản trước.",
			"This account is disabled. Enable it first."))
}

func staleWrite(ctx context.Context, err error) openapi.ErrorResponse {
	if errors.Is(err, domain.ErrWriteContended) {
		return httpapi.Error(ctx, openapi.STALEWRITE,
			httpx.Text(ctx, "Tài khoản này đang được thay đổi cùng lúc. Hãy thử lại.", "This account was being changed at the same moment. Try again."))
	}
	return httpapi.Error(ctx, openapi.STALEWRITE,
		httpx.Text(ctx, "Vai trò vừa được thay đổi. Hãy thử lại.", "The role was just changed. Try again."))
}

func isStale(err error) bool {
	return errors.Is(err, domain.ErrRoleChanged) || errors.Is(err, domain.ErrWriteContended)
}

func failureCode(reason error) openapi.ErrorCode {
	switch {
	case errors.Is(reason, domain.ErrUserNotFound):
		return openapi.NOTFOUND
	case errors.Is(reason, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.TARGETROLENOTALLOWED
	case errors.Is(reason, domain.ErrCannotActOnSelf):
		return openapi.CANNOTACTONSELF
	case errors.Is(reason, domain.ErrLastAdmin):
		return openapi.LASTADMIN
	case errors.Is(reason, domain.ErrAccountDisabled):
		return openapi.ACCOUNTDISABLED
	case isStale(reason):
		return openapi.STALEWRITE
	default:
		return openapi.INTERNAL
	}
}

func userFailures(failed []domain.UserFailure) []openapi.UserActionFailure {
	out := make([]openapi.UserActionFailure, len(failed))
	for i, failure := range failed {
		out[i] = openapi.UserActionFailure{UserId: httpapi.ParseUUID(failure.UserID), Code: failureCode(failure.Reason)}
	}
	return out
}

// ChangeUserRole implements PUT /admin/users/{id}/role.
func (h Identity) ChangeUserRole(ctx context.Context, request openapi.ChangeUserRoleRequestObject) (openapi.ChangeUserRoleResponseObject, error) {
	if h.app == nil || h.app.Commands.ChangeUserRole == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	user, err := h.app.Commands.ChangeUserRole.Handle(ctx, command.ChangeUserRole{Request: req, ID: request.Id.String(), RoleID: request.Body.RoleId.String()})
	var owns *domain.OwnsContentError
	switch {
	case err == nil:
		return openapi.ChangeUserRole200JSONResponse(toAPIAdminUser(user)), nil
	case errors.Is(err, domain.ErrUserNotFound):
		return openapi.ChangeUserRole404JSONResponse{NotFoundJSONResponse: userNotFound(ctx)}, nil
	case errors.Is(err, accessdomain.ErrRoleNotFound):
		return openapi.ChangeUserRole404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy vai trò này.", "No such role.")))}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.ChangeUserRole403JSONResponse(targetNotAllowed(ctx)), nil
	case errors.Is(err, domain.ErrCannotActOnSelf):
		return openapi.ChangeUserRole422JSONResponse(cannotActOnSelf(ctx)), nil
	case errors.Is(err, accessdomain.ErrNotAssignable):
		return openapi.ChangeUserRole422JSONResponse(httpapi.Error(ctx, openapi.ROLENOTASSIGNABLE,
			httpx.Text(ctx, "Không thể chọn vai trò này.", "This role cannot be chosen."))), nil
	case errors.Is(err, domain.ErrLastAdmin):
		return openapi.ChangeUserRole409JSONResponse(lastAdmin(ctx)), nil
	case errors.As(err, &owns):
		return openapi.ChangeUserRole409JSONResponse(httpapi.ErrorWithDetails(ctx, openapi.USEROWNSCONTENT,
			httpx.Text(ctx, "Tài khoản này đang sở hữu lớp học hoặc nội dung. Hãy chuyển nội dung đi trước khi đổi sang vai trò này.",
				"This account owns classes or content. Transfer it before giving the account this role."),
			map[string]interface{}{
				"classes":     owns.Owned.Classes,
				"tests":       owns.Owned.Tests,
				"questions":   owns.Owned.Questions,
				"media":       owns.Owned.Media,
				"assignments": owns.Owned.Assignments,
			})), nil
	case isStale(err):
		return openapi.ChangeUserRole409JSONResponse(staleWrite(ctx, err)), nil
	default:
		return nil, err
	}
}

// DisableUser implements POST /admin/users/{id}/disable.
func (h Identity) DisableUser(ctx context.Context, request openapi.DisableUserRequestObject) (openapi.DisableUserResponseObject, error) {
	if h.app == nil || h.app.Commands.DisableUser == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	user, err := h.app.Commands.DisableUser.Handle(ctx, command.DisableUser{Request: req, ID: request.Id.String()})
	switch {
	case err == nil:
		return openapi.DisableUser200JSONResponse(toAPIAdminUser(user)), nil
	case errors.Is(err, domain.ErrUserNotFound):
		return openapi.DisableUser404JSONResponse{NotFoundJSONResponse: userNotFound(ctx)}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.DisableUser403JSONResponse(targetNotAllowed(ctx)), nil
	case errors.Is(err, domain.ErrCannotActOnSelf):
		return openapi.DisableUser422JSONResponse(cannotActOnSelf(ctx)), nil
	case errors.Is(err, domain.ErrLastAdmin):
		return openapi.DisableUser409JSONResponse(lastAdmin(ctx)), nil
	case isStale(err):
		return openapi.DisableUser409JSONResponse(staleWrite(ctx, err)), nil
	default:
		return nil, err
	}
}

// DisableUsers implements POST /admin/users/disable.
func (h Identity) DisableUsers(ctx context.Context, request openapi.DisableUsersRequestObject) (openapi.DisableUsersResponseObject, error) {
	if h.app == nil || h.app.Commands.DisableUsers == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	if httpapi.RepeatsAnID(request.Body.UserIds) {
		return openapi.DisableUsers400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "userIds", httpx.Text(ctx, "Mỗi tài khoản chỉ được nêu một lần.", "Each account can be named only once.")))}, nil
	}
	result, err := h.app.Commands.DisableUsers.Handle(ctx, command.DisableUsers{Request: req, IDs: userIDs(request.Body.UserIds)})
	if err != nil {
		return nil, err
	}
	out := openapi.DisableUsers200JSONResponse{Disabled: make([]openapi.Uuid, len(result.Disabled)), Failed: userFailures(result.Failed)}
	for i, id := range result.Disabled {
		out.Disabled[i] = httpapi.ParseUUID(id)
	}
	return out, nil
}

// EnableUser implements POST /admin/users/{id}/enable.
func (h Identity) EnableUser(ctx context.Context, request openapi.EnableUserRequestObject) (openapi.EnableUserResponseObject, error) {
	if h.app == nil || h.app.Commands.EnableUser == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	user, err := h.app.Commands.EnableUser.Handle(ctx, command.EnableUser{Request: req, ID: request.Id.String()})
	switch {
	case err == nil:
		return openapi.EnableUser200JSONResponse(toAPIAdminUser(user)), nil
	case errors.Is(err, domain.ErrUserNotFound):
		return openapi.EnableUser404JSONResponse{NotFoundJSONResponse: userNotFound(ctx)}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.EnableUser403JSONResponse(targetNotAllowed(ctx)), nil
	case isStale(err):
		return openapi.EnableUser409JSONResponse(staleWrite(ctx, err)), nil
	default:
		return nil, err
	}
}

// SignOutUser implements POST /admin/users/{id}/sign-out.
func (h Identity) SignOutUser(ctx context.Context, request openapi.SignOutUserRequestObject) (openapi.SignOutUserResponseObject, error) {
	if h.app == nil || h.app.Commands.SignOutUser == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.SignOutUser.Handle(ctx, command.SignOutUser{Request: req, ID: request.Id.String()})
	switch {
	case err == nil:
		return openapi.SignOutUser204Response{}, nil
	case errors.Is(err, domain.ErrUserNotFound):
		return openapi.SignOutUser404JSONResponse{NotFoundJSONResponse: userNotFound(ctx)}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.SignOutUser403JSONResponse(targetNotAllowed(ctx)), nil
	case isStale(err):
		return openapi.SignOutUser409JSONResponse(staleWrite(ctx, err)), nil
	default:
		return nil, err
	}
}

// ResetUserPassword implements POST /admin/users/{id}/reset-password.
func (h Identity) ResetUserPassword(ctx context.Context, request openapi.ResetUserPasswordRequestObject) (openapi.ResetUserPasswordResponseObject, error) {
	if h.app == nil || h.app.Commands.ResetUserPassword == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	result, err := h.app.Commands.ResetUserPassword.Handle(ctx, command.ResetUserPassword{
		Request: req, ID: request.Id.String(), Delivery: domain.Delivery(request.Body.Delivery),
	})
	switch {
	case err == nil:
		out := openapi.ResetUserPassword200JSONResponse{}
		out.Headers.CacheControl = httpapi.Ptr("no-store")
		if result.TemporaryPassword != "" {
			out.Body.TemporaryPassword = httpapi.Ptr(result.TemporaryPassword)
		}
		return out, nil
	case errors.Is(err, domain.ErrPasswordSignInDisabled):
		return openapi.ResetUserPassword422JSONResponse(passwordSignInOff(ctx)), nil
	case errors.Is(err, domain.ErrSetPasswordUnavailable):
		return nil, httpx.ErrNotImplemented
	case errors.Is(err, domain.ErrUserNotFound):
		return openapi.ResetUserPassword404JSONResponse{NotFoundJSONResponse: userNotFound(ctx)}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.ResetUserPassword403JSONResponse(targetNotAllowed(ctx)), nil
	case errors.Is(err, domain.ErrCannotActOnSelf):
		return openapi.ResetUserPassword422JSONResponse(cannotActOnSelf(ctx)), nil
	case errors.Is(err, domain.ErrAccountDisabled):
		return openapi.ResetUserPassword409JSONResponse(accountDisabled(ctx)), nil
	case isStale(err):
		return openapi.ResetUserPassword409JSONResponse(staleWrite(ctx, err)), nil
	default:
		return nil, err
	}
}

// ResetUsersPasswords implements POST /admin/users/reset-passwords.
func (h Identity) ResetUsersPasswords(ctx context.Context, request openapi.ResetUsersPasswordsRequestObject) (openapi.ResetUsersPasswordsResponseObject, error) {
	if h.app == nil || h.app.Commands.ResetUsersPasswords == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	if httpapi.RepeatsAnID(request.Body.UserIds) {
		return openapi.ResetUsersPasswords400JSONResponse(httpapi.FieldError(ctx, "userIds",
			httpx.Text(ctx, "Mỗi tài khoản chỉ được nêu một lần.", "Each account can be named only once."))), nil
	}
	result, err := h.app.Commands.ResetUsersPasswords.Handle(ctx, command.ResetUsersPasswords{
		Request: req, IDs: userIDs(request.Body.UserIds), Delivery: domain.Delivery(request.Body.Delivery),
	})
	var invalid *validation.Error
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.ResetUsersPasswords400JSONResponse(httpapi.Invalid(ctx,
			httpx.Text(ctx, "Danh sách tài khoản không hợp lệ.", "The list of accounts is not valid."), invalid)), nil
	case errors.Is(err, domain.ErrPasswordSignInDisabled):
		return openapi.ResetUsersPasswords422JSONResponse(passwordSignInOff(ctx)), nil
	case errors.Is(err, domain.ErrSetPasswordUnavailable):
		return nil, httpx.ErrNotImplemented
	default:
		return nil, err
	}

	var out openapi.ResetUsersPasswords200JSONResponse
	out.Headers.CacheControl = httpapi.Ptr("no-store")
	out.Body.Items = make([]openapi.UserPasswordReset, len(result.Reset))
	for i, reset := range result.Reset {
		out.Body.Items[i] = openapi.UserPasswordReset{
			UserId:   httpapi.ParseUUID(reset.UserID),
			FullName: reset.FullName,
			Email:    openapi_types.Email(reset.Email),
		}
		if reset.TemporaryPassword != "" {
			out.Body.Items[i].TemporaryPassword = httpapi.Ptr(reset.TemporaryPassword)
		}
	}
	out.Body.Failed = userFailures(result.Failed)
	return out, nil
}

// TransferUserContent implements POST /admin/users/{id}/transfer-content.
func (h Identity) TransferUserContent(ctx context.Context, request openapi.TransferUserContentRequestObject) (openapi.TransferUserContentResponseObject, error) {
	if h.app == nil || h.app.Commands.TransferUserContent == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := studentRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	notify := request.Body.Notify != nil && *request.Body.Notify
	result, err := h.app.Commands.TransferUserContent.Handle(ctx, command.TransferUserContent{
		Request: req, ID: request.Id.String(), ToID: request.Body.ToUserId.String(), Notify: notify,
	})
	switch {
	case err == nil:
		return openapi.TransferUserContent200JSONResponse(toAPITransferred(result)), nil
	case errors.Is(err, domain.ErrUserNotFound):
		return openapi.TransferUserContent404JSONResponse{NotFoundJSONResponse: userNotFound(ctx)}, nil
	case errors.Is(err, accessdomain.ErrTargetRoleNotAllowed):
		return openapi.TransferUserContent403JSONResponse(targetNotAllowed(ctx)), nil
	case errors.Is(err, domain.ErrUserNotDisabled):
		return openapi.TransferUserContent409JSONResponse(httpapi.Error(ctx, openapi.USERNOTDISABLED,
			httpx.Text(ctx, "Hãy vô hiệu hoá tài khoản này trước khi chuyển nội dung của nó.",
				"Disable this account before you transfer its content."))), nil
	case isStale(err):
		return openapi.TransferUserContent409JSONResponse(staleWrite(ctx, err)), nil
	case errors.Is(err, domain.ErrInvalidRecipient):
		return openapi.TransferUserContent422JSONResponse(httpapi.Error(ctx, openapi.INVALIDRECIPIENT,
			httpx.Text(ctx, "Người nhận phải là một giáo viên khác đang hoạt động.",
				"The recipient must be another active teacher."))), nil
	default:
		return nil, err
	}
}

func toAPITransferred(r domain.TransferredContent) openapi.UserContentTransferResult {
	var out openapi.UserContentTransferResult
	out.Moved.Classes = r.Moved.Classes
	out.Moved.Tests = r.Moved.Tests
	out.Moved.Questions = r.Moved.Questions
	out.Moved.QuestionGroups = r.Moved.QuestionGroups
	out.Moved.Media = r.Moved.Media
	out.Moved.Assignments = r.Moved.Assignments
	out.LeftBehind.Imports = r.LeftBehind.Imports
	out.LeftBehind.Students = r.LeftBehind.Students
	return out
}
