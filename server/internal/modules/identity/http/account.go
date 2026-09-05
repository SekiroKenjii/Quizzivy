package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// GetCurrentUser implements GET /auth/me (§5.4).
func (h Identity) GetCurrentUser(ctx context.Context, _ openapi.GetCurrentUserRequestObject) (openapi.GetCurrentUserResponseObject, error) {
	if h.auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.GetCurrentUser401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	}

	user, err := h.auth.CurrentUser(ctx, principal.UserID)
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

// ChangePassword implements POST /auth/change-password (§5.4).
func (h Identity) ChangePassword(ctx context.Context, request openapi.ChangePasswordRequestObject) (openapi.ChangePasswordResponseObject, error) {
	if h.auth == nil {
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
	err := h.auth.ChangePassword(ctx, application.ChangePasswordInput{
		UserID:           principal.UserID,
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
