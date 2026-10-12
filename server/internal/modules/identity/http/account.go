package http

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// GetCurrentUser implements GET /auth/me (§5.4).
func (h Identity) GetCurrentUser(ctx context.Context, _ openapi.GetCurrentUserRequestObject) (openapi.GetCurrentUserResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.GetCurrentUser401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	}

	user, err := h.app.Queries.CurrentUser.Handle(ctx, query.CurrentUser{UserID: principal.UserID})
	if err != nil {
		if errors.Is(err, domain.ErrAccountDisabled) || errors.Is(err, domain.ErrUserNotFound) {
			return openapi.GetCurrentUser401JSONResponse{
				UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
			}, nil
		}
		return nil, err
	}

	return openapi.GetCurrentUser200JSONResponse(h.currentUser(ctx, user, principal.Access.Permissions)), nil
}

// UpdateCurrentUser applies the caller's supplied profile fields.
func (h Identity) UpdateCurrentUser(ctx context.Context, request openapi.UpdateCurrentUserRequestObject) (openapi.UpdateCurrentUserResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.UpdateCurrentUser401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	if request.Body == nil {
		return openapi.UpdateCurrentUser400JSONResponse(profileError(ctx, domain.ErrProfileEmpty)), nil
	}
	patch, err := profilePatch(*request.Body)
	if err != nil {
		return openapi.UpdateCurrentUser400JSONResponse(profileError(ctx, err)), nil
	}
	meta := httpx.RequestMetaFromContext(ctx)
	user, err := h.app.Commands.UpdateProfile.Handle(ctx, command.UpdateProfile{UserID: principal.UserID, Patch: patch, IP: meta.IP, UserAgent: meta.UserAgent})
	if err == nil {
		return openapi.UpdateCurrentUser200JSONResponse(h.currentUser(ctx, user, principal.Access.Permissions)), nil
	}
	if errors.Is(err, domain.ErrAccountDisabled) || errors.Is(err, domain.ErrUserNotFound) {
		return openapi.UpdateCurrentUser401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	if profileValidation(err) {
		return openapi.UpdateCurrentUser400JSONResponse(profileError(ctx, err)), nil
	}
	return nil, err
}

func profilePatch(body openapi.UpdateCurrentUserJSONRequestBody) (domain.ProfilePatch, error) {
	p := domain.ProfilePatch{FullName: body.FullName, DisplayNameSet: len(body.DisplayName) > 0, PhoneSet: len(body.Phone) > 0, Locale: (*string)(body.Locale), TimeZone: body.TimeZone}
	if p.DisplayNameSet {
		if err := json.Unmarshal(body.DisplayName, &p.DisplayName); err != nil {
			return p, domain.ErrDisplayNameInvalid
		}
	}
	if p.PhoneSet {
		if err := json.Unmarshal(body.Phone, &p.Phone); err != nil {
			return p, domain.ErrPhoneInvalid
		}
	}
	return p, nil
}

func profileValidation(err error) bool {
	return errors.Is(err, domain.ErrProfileEmpty) || errors.Is(err, domain.ErrNameRequired) || errors.Is(err, domain.ErrNameTooLong) || errors.Is(err, domain.ErrDisplayNameInvalid) || errors.Is(err, domain.ErrPhoneInvalid) || errors.Is(err, domain.ErrLocaleInvalid) || errors.Is(err, domain.ErrTimeZoneInvalid)
}

func profileError(ctx context.Context, err error) openapi.ErrorResponse {
	switch {
	case errors.Is(err, domain.ErrNameRequired):
		return httpapi.FieldError(ctx, "fullName", httpx.Text(ctx, "Họ và tên không được để trống.", "Enter your name."))
	case errors.Is(err, domain.ErrNameTooLong):
		return httpapi.FieldError(ctx, "fullName", httpx.Text(ctx, "Họ và tên quá dài.", "The name is too long."))
	case errors.Is(err, domain.ErrDisplayNameInvalid):
		return httpapi.FieldError(ctx, "displayName", httpx.Text(ctx, "Tên hiển thị không hợp lệ.", "The display name is not valid."))
	case errors.Is(err, domain.ErrPhoneInvalid):
		return httpapi.FieldError(ctx, "phone", httpx.Text(ctx, "Số điện thoại không hợp lệ.", "The phone number is not valid."))
	case errors.Is(err, domain.ErrLocaleInvalid):
		return httpapi.FieldError(ctx, "locale", httpx.Text(ctx, "Ngôn ngữ không hợp lệ.", "The language is not valid."))
	case errors.Is(err, domain.ErrTimeZoneInvalid):
		return httpapi.FieldError(ctx, "timeZone", httpx.Text(ctx, "Múi giờ không hợp lệ.", "The time zone is not valid."))
	default:
		return httpapi.Error(ctx, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Thiếu thông tin hồ sơ.", "Supply a profile field."))
	}
}

// ChangePassword implements POST /auth/change-password (§5.4).
func (h Identity) ChangePassword(ctx context.Context, request openapi.ChangePasswordRequestObject) (openapi.ChangePasswordResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.ChangePassword401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	}
	if request.Body == nil {
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Thiếu thông tin mật khẩu.", "The password details are missing."))), nil
	}

	meta := httpx.RequestMetaFromContext(ctx)
	_, err := h.app.Commands.ChangePassword.Handle(ctx, command.ChangePassword{UserID: principal.UserID,
		CurrentPassword:  httpapi.DerefString(request.Body.CurrentPassword),
		NewPassword:      request.Body.NewPassword,
		KeepRefreshToken: refreshTokenFromContext(ctx),
		IP:               meta.IP,
		UserAgent:        meta.UserAgent,
	})

	switch {
	case err == nil:
		return openapi.ChangePassword204Response{}, nil

	case errors.Is(err, domain.ErrInvalidCredentials):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.INVALIDCREDENTIALS,
			httpx.Text(ctx, "Mật khẩu hiện tại không đúng.", "The current password is incorrect."))), nil

	case errors.Is(err, domain.ErrNoPasswordSet):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.PASSWORDREQUIRED,
			httpx.Text(ctx, "Tài khoản này đăng nhập bằng Google và chưa có mật khẩu.",
				"This account signs in with Google and has no password yet."))), nil

	case errors.Is(err, domain.ErrPasswordUnchanged):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.PASSWORDUNCHANGED,
			httpx.Text(ctx, "Mật khẩu mới phải khác mật khẩu hiện tại.",
				"The new password must be different from the current one."))), nil

	case errors.Is(err, domain.ErrPasswordTooShort):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Mật khẩu mới phải có ít nhất 8 ký tự.",
				"The new password must have at least 8 characters."))), nil

	case errors.Is(err, domain.ErrPasswordTooLong):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Mật khẩu mới quá dài.", "The new password is too long."))), nil

	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.ChangePassword401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil

	default:
		return nil, err
	}
}

func sessionInvalid(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.UNAUTHORIZED,
		httpx.Text(ctx, "Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.",
			"Your session is not valid. Please sign in again."))
}
