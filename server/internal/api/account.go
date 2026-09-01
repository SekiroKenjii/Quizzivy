package api

import (
	"context"
	"errors"

	"quizzivy/gen/openapi"
	"quizzivy/internal/auth"
	"quizzivy/internal/httpx"
)

// GetCurrentUser implements GET /auth/me (§5.4).
//
// The SPA calls this on every load to decide between the app and /login, so it
// re-reads the user rather than reflecting the token's claims back. A student
// suspended five minutes ago still holds a valid access token; this is where
// that stops working.
func (s *Server) GetCurrentUser(ctx context.Context, _ openapi.GetCurrentUserRequestObject) (openapi.GetCurrentUserResponseObject, error) {
	if s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.GetCurrentUser401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	}

	user, err := s.Deps.Auth.CurrentUser(ctx, principal.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrAccountDisabled) || errors.Is(err, auth.ErrUserNotFound) {
			return openapi.GetCurrentUser401JSONResponse{
				UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
			}, nil
		}
		return nil, err
	}

	return openapi.GetCurrentUser200JSONResponse(toAPIUser(user)), nil
}

// ChangePassword implements POST /auth/change-password (§5.4).
//
// Every rejection that is about the SUBMITTED password is a 400, never a 401.
// The SPA treats 401 as a dead session: it refreshes once, retries, and signs
// the user out on the second 401 (client.ts). A 401 here would mean mistyping
// your own current password silently signs you out, with nothing pointing at
// the typo. 401 on this endpoint means the session is invalid, and nothing else.
func (s *Server) ChangePassword(ctx context.Context, request openapi.ChangePasswordRequestObject) (openapi.ChangePasswordResponseObject, error) {
	if s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.ChangePassword401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	}
	if request.Body == nil {
		return openapi.ChangePassword400JSONResponse(authError(ctx, openapi.VALIDATIONFAILED,
			"Thiếu thông tin mật khẩu.")), nil
	}

	meta := httpx.RequestMetaFromContext(ctx)
	err := s.Deps.Auth.ChangePassword(ctx, auth.ChangePasswordInput{
		UserID:           principal.UserID,
		CurrentPassword:  derefString(request.Body.CurrentPassword),
		NewPassword:      request.Body.NewPassword,
		KeepRefreshToken: refreshTokenFromContext(ctx),
		IP:               meta.IP,
		UserAgent:        meta.UserAgent,
	})

	switch {
	case err == nil:
		return openapi.ChangePassword204Response{}, nil

	case errors.Is(err, auth.ErrInvalidCredentials):
		return openapi.ChangePassword400JSONResponse(authError(ctx, openapi.INVALIDCREDENTIALS,
			"Mật khẩu hiện tại không đúng.")), nil

	case errors.Is(err, auth.ErrNoPasswordSet):
		return openapi.ChangePassword400JSONResponse(authError(ctx, openapi.PASSWORDREQUIRED,
			"Tài khoản này đăng nhập bằng Google và chưa có mật khẩu.")), nil

	case errors.Is(err, auth.ErrPasswordTooShort):
		return openapi.ChangePassword400JSONResponse(authError(ctx, openapi.VALIDATIONFAILED,
			"Mật khẩu mới phải có ít nhất 8 ký tự.")), nil

	case errors.Is(err, auth.ErrPasswordTooLong):
		return openapi.ChangePassword400JSONResponse(authError(ctx, openapi.VALIDATIONFAILED,
			"Mật khẩu mới quá dài.")), nil

	case errors.Is(err, auth.ErrAccountDisabled), errors.Is(err, auth.ErrUserNotFound):
		return openapi.ChangePassword401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil

	default:
		return nil, err
	}
}

func sessionInvalid(ctx context.Context) openapi.ErrorResponse {
	return authError(ctx, openapi.UNAUTHORIZED, "Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.")
}

// derefString reads an optional body field. `currentPassword` is absent exactly
// when the change is a forced one, which auth.ChangePassword handles.
func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
