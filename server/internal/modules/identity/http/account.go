package http

import (
	"context"
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

	return openapi.GetCurrentUser200JSONResponse(toAPIUser(user)), nil
}

// UpdateCurrentUser implements PATCH /auth/me: the "Hồ sơ" card's save.
func (h Identity) UpdateCurrentUser(ctx context.Context, request openapi.UpdateCurrentUserRequestObject) (openapi.UpdateCurrentUserResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.UpdateCurrentUser401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	}
	if request.Body == nil {
		return openapi.UpdateCurrentUser400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			"Thiếu họ và tên.")), nil
	}

	meta := httpx.RequestMetaFromContext(ctx)
	user, err := h.app.Commands.Rename.Handle(ctx, command.Rename{
		UserID:    principal.UserID,
		FullName:  request.Body.FullName,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.UpdateCurrentUser200JSONResponse(toAPIUser(user)), nil

	case errors.Is(err, domain.ErrNameRequired):
		return openapi.UpdateCurrentUser400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			"Họ và tên không được để trống.")), nil

	case errors.Is(err, domain.ErrNameTooLong):
		return openapi.UpdateCurrentUser400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			"Họ và tên quá dài.")), nil

	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.UpdateCurrentUser401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil

	default:
		return nil, err
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
			"Thiếu thông tin mật khẩu.")), nil
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
			"Mật khẩu hiện tại không đúng.")), nil

	case errors.Is(err, domain.ErrNoPasswordSet):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.PASSWORDREQUIRED,
			"Tài khoản này đăng nhập bằng Google và chưa có mật khẩu.")), nil

	case errors.Is(err, domain.ErrPasswordTooShort):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			"Mật khẩu mới phải có ít nhất 8 ký tự.")), nil

	case errors.Is(err, domain.ErrPasswordTooLong):
		return openapi.ChangePassword400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			"Mật khẩu mới quá dài.")), nil

	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.ChangePassword401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil

	default:
		return nil, err
	}
}

func sessionInvalid(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.UNAUTHORIZED, "Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.")
}
