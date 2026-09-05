package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Login implements POST /auth/login (§5.1).
//
// Every failure returns the same 401 with the same code and the same message.
// §6.5 requires the join endpoints not to reveal which classes exist; the same
// reasoning applies here to which accounts exist and which are suspended. The
// service layer additionally equalises the TIMING, which a response body alone
// cannot do.
func (h Identity) Login(ctx context.Context, request openapi.LoginRequestObject) (openapi.LoginResponseObject, error) {
	if h.auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return openapi.Login401JSONResponse(invalidCredentials(ctx)), nil
	}

	meta := httpx.RequestMetaFromContext(ctx)
	session, err := h.auth.Login(ctx, application.LoginInput{
		Email:     string(request.Body.Email),
		Password:  request.Body.Password,
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return openapi.Login401JSONResponse(invalidCredentials(ctx)), nil
		}
		return nil, err
	}

	return openapi.Login200JSONResponse{
		Body: openapi.AuthSuccess{
			AccessToken: session.AccessToken,
			ExpiresIn:   session.ExpiresIn,
			User:        toAPIUser(session.User),
		},
		Headers: openapi.Login200ResponseHeaders{
			SetCookie: httpapi.Ptr(refreshCookie(session.RefreshToken, h.refreshTTL, h.cookieSecure).String()),
		},
	}, nil
}

func invalidCredentials(ctx context.Context) openapi.ErrorResponse {
	return openapi.ErrorResponse{
		Error: struct {
			Code      openapi.ErrorCode       `json:"code"`
			Details   *map[string]interface{} `json:"details,omitempty"`
			Message   string                  `json:"message"`
			RequestId openapi.Uuid            `json:"requestId"`
		}{
			Code:      openapi.INVALIDCREDENTIALS,
			Message:   "Email hoặc mật khẩu không đúng.",
			RequestId: httpapi.ParseUUID(httpx.RequestIDFromContext(ctx)),
		},
	}
}

// RefreshSession implements POST /auth/refresh (§5.2).
//
// The rotated cookie is the whole point of the response: the predecessor is
// revoked server-side before this returns, so a client that does not receive
// the replacement is already logged out and does not know it yet.
func (h Identity) RefreshSession(ctx context.Context, _ openapi.RefreshSessionRequestObject) (openapi.RefreshSessionResponseObject, error) {
	if h.auth == nil {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	res, err := h.auth.Refresh(ctx, application.RefreshInput{
		Token:     refreshTokenFromContext(ctx),
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
	})
	switch {
	case errors.Is(err, domain.ErrRefreshReused):
		return openapi.RefreshSession401JSONResponse(httpapi.Error(ctx, openapi.REFRESHTOKENREUSED,
			"Phiên đăng nhập này đã được sử dụng ở nơi khác. Vì lý do an toàn, vui lòng đăng nhập lại.")), nil
	case errors.Is(err, domain.ErrRefreshRejected):
		return openapi.RefreshSession401JSONResponse(httpapi.Error(ctx, openapi.REFRESHTOKENINVALID,
			"Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.")), nil
	case err != nil:
		return nil, err
	}

	var body openapi.RefreshSession200JSONResponse
	body.Body.AccessToken = res.AccessToken
	body.Body.ExpiresIn = res.ExpiresIn
	body.Headers.SetCookie = httpapi.Ptr(refreshCookie(res.RefreshToken, h.refreshTTL, h.cookieSecure).String())
	return body, nil
}

// Logout implements POST /auth/logout (§5.4).
//
// Authenticated by the refresh cookie rather than the access token: a user
// whose access token has already expired must still be able to end their
// session, and that is precisely when they are most likely to try.
func (h Identity) Logout(ctx context.Context, _ openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	if h.auth == nil {
		return nil, httpx.ErrNotImplemented
	}

	token := refreshTokenFromContext(ctx)
	if token == "" {
		return openapi.Logout401JSONResponse(httpapi.Error(ctx, openapi.REFRESHTOKENINVALID,
			"Không có phiên đăng nhập.")), nil
	}
	if err := h.auth.Logout(ctx, token); err != nil {
		return nil, err
	}

	return openapi.Logout204Response{
		Headers: openapi.Logout204ResponseHeaders{
			SetCookie: httpapi.Ptr(clearRefreshCookie(h.cookieSecure).String()),
		},
	}, nil
}

func toAPIUser(u domain.User) openapi.User {
	providers := make([]openapi.UserLinkedProviders, 0, len(u.LinkedProviders))
	for _, p := range u.LinkedProviders {
		providers = append(providers, openapi.UserLinkedProviders(p))
	}
	return openapi.User{
		Id:                 httpapi.ParseUUID(u.ID),
		Email:              openapi_types.Email(u.Email),
		FullName:           u.FullName,
		Role:               openapi.Role(u.Role),
		HasPassword:        u.HasPassword(),
		LinkedProviders:    providers,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt,
	}
}
