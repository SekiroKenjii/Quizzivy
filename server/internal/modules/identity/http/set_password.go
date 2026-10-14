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

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// CheckSetPasswordLink implements POST /auth/set-password/check.
func (h Identity) CheckSetPasswordLink(ctx context.Context, request openapi.CheckSetPasswordLinkRequestObject) (openapi.CheckSetPasswordLinkResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return openapi.CheckSetPasswordLink410JSONResponse(setPasswordLinkInvalid(ctx)), nil
	}
	link, err := h.app.Queries.CheckSetPasswordLink.Handle(ctx, query.CheckSetPasswordLink{Token: request.Body.Token})
	if errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		return openapi.CheckSetPasswordLink410JSONResponse(setPasswordLinkInvalid(ctx)), nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.CheckSetPasswordLink200JSONResponse{
		Email:     openapi_types.Email(link.Email),
		FullName:  link.FullName,
		Purpose:   openapi.SetPasswordLinkInfoPurpose(link.Purpose),
		ExpiresAt: link.ExpiresAt,
	}, nil
}

// SetPasswordWithLink implements POST /auth/set-password.
func (h Identity) SetPasswordWithLink(ctx context.Context, request openapi.SetPasswordWithLinkRequestObject) (openapi.SetPasswordWithLinkResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return openapi.SetPasswordWithLink410JSONResponse(setPasswordLinkInvalid(ctx)), nil
	}
	meta := httpx.RequestMetaFromContext(ctx)
	session, err := h.app.Commands.SetPasswordWithLink.Handle(ctx, command.SetPasswordWithLink{
		Token:       request.Body.Token,
		NewPassword: request.Body.NewPassword,
		UserAgent:   meta.UserAgent,
		IP:          meta.IP,
		GeoLabel:    GeoLabelFromContext(ctx),
	})
	switch {
	case err == nil:
		return openapi.SetPasswordWithLink200JSONResponse{
			Body: openapi.AuthSuccess{
				AccessToken: session.AccessToken,
				ExpiresIn:   session.ExpiresIn,
				User:        h.currentUser(ctx, session.User, session.Permissions),
			},
			Headers: openapi.SetPasswordWithLink200ResponseHeaders{
				SetCookie: httpapi.Ptr(refreshCookie(session.RefreshToken, h.refreshTTL, h.cookieSecure).String()),
			},
		}, nil
	case errors.Is(err, domain.ErrSetPasswordLinkInvalid):
		return openapi.SetPasswordWithLink410JSONResponse(setPasswordLinkInvalid(ctx)), nil
	case errors.Is(err, domain.ErrPasswordSignInDisabled):
		return openapi.SetPasswordWithLink422JSONResponse(passwordSignInOff(ctx)), nil
	case errors.Is(err, domain.ErrPasswordTooShort):
		return openapi.SetPasswordWithLink400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, "newPassword",
			httpx.Text(ctx, "Mật khẩu mới phải có ít nhất 8 ký tự.", "The new password must have at least 8 characters.")))}, nil
	case errors.Is(err, domain.ErrPasswordTooLong):
		return openapi.SetPasswordWithLink400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, "newPassword",
			httpx.Text(ctx, "Mật khẩu mới quá dài.", "The new password is too long.")))}, nil
	default:
		return nil, err
	}
}

// RequestSetPasswordLink implements POST /auth/set-password/request.
func (h Identity) RequestSetPasswordLink(ctx context.Context, _ openapi.RequestSetPasswordLinkRequestObject) (openapi.RequestSetPasswordLinkResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.RequestSetPasswordLink401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	meta := httpx.RequestMetaFromContext(ctx)
	_, err := h.app.Commands.RequestSetPasswordLink.Handle(ctx, command.RequestSetPasswordLink{
		UserID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.RequestSetPasswordLink204Response{}, nil
	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.RequestSetPasswordLink401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	case errors.Is(err, domain.ErrPasswordSignInDisabled):
		return openapi.RequestSetPasswordLink422JSONResponse(passwordSignInOff(ctx)), nil
	case errors.Is(err, domain.ErrSetPasswordUnavailable):
		return nil, httpx.ErrNotImplemented
	default:
		return nil, err
	}
}

func setPasswordLinkInvalid(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.SETPASSWORDLINKINVALID,
		httpx.Text(ctx, "Liên kết này đã hết hạn hoặc đã được dùng. Hãy nhờ trung tâm gửi liên kết mới.",
			"This link has expired or has already been used. Ask your centre for a new one."))
}

func passwordSignInOff(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.PASSWORDSIGNINDISABLED,
		httpx.Text(ctx, "Trung tâm đã tắt đăng nhập bằng mật khẩu. Hãy đăng nhập bằng Google.",
			"Your centre has turned off password sign-in. Sign in with Google."))
}
