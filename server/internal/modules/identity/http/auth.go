package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Login implements POST /auth/login (§5.1).
func (h Identity) Login(ctx context.Context, request openapi.LoginRequestObject) (openapi.LoginResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return openapi.Login401JSONResponse(invalidCredentials(ctx)), nil
	}

	meta := httpx.RequestMetaFromContext(ctx)
	session, err := h.app.Commands.Login.Handle(ctx, command.Login{Email: string(request.Body.Email),
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
			User:        toCurrentUser(session.User, session.Permissions),
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
			Message:   httpx.Text(ctx, "Email hoặc mật khẩu không đúng.", "The email or password is incorrect."),
			RequestId: httpapi.ParseUUID(httpx.RequestIDFromContext(ctx)),
		},
	}
}

// RefreshSession implements POST /auth/refresh (§5.2).
func (h Identity) RefreshSession(ctx context.Context, _ openapi.RefreshSessionRequestObject) (openapi.RefreshSessionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	res, err := h.app.Commands.Refresh.Handle(ctx, command.Refresh{Token: refreshTokenFromContext(ctx),
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
	})
	switch {
	case errors.Is(err, domain.ErrRefreshReused):
		return openapi.RefreshSession401JSONResponse(httpapi.Error(ctx, openapi.REFRESHTOKENREUSED,
			httpx.Text(ctx, "Phiên đăng nhập này đã được sử dụng ở nơi khác. Vì lý do an toàn, vui lòng đăng nhập lại.",
				"This session was used somewhere else. For your safety, please sign in again."))), nil
	case errors.Is(err, domain.ErrRefreshRejected):
		return openapi.RefreshSession401JSONResponse(httpapi.Error(ctx, openapi.REFRESHTOKENINVALID,
			httpx.Text(ctx, "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
				"Your session has expired. Please sign in again."))), nil
	case err != nil:
		return nil, err
	}

	var body openapi.RefreshSession200JSONResponse
	body.Body.AccessToken = res.AccessToken
	body.Body.ExpiresIn = res.ExpiresIn
	body.Headers.SetCookie = httpapi.Ptr(refreshCookie(res.RefreshToken, h.refreshTTL, h.cookieSecure).String())
	return body, nil
}

// Logout implements POST /auth/logout (§5.4). When the revoke fails the answer
// is the router's 500 and still clears both cookies.
func (h Identity) Logout(ctx context.Context, _ openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	token := refreshTokenFromContext(ctx)
	if token == "" {
		return openapi.Logout401JSONResponse(httpapi.Error(ctx, openapi.REFRESHTOKENINVALID,
			httpx.Text(ctx, "Không có phiên đăng nhập.", "There is no sign-in session."))), nil
	}
	if _, err := h.app.Commands.Logout.Handle(ctx, command.Logout{Token: token}); err != nil {
		return failedLogout{cleared: clearedSession{clearRefreshCookie(h.cookieSecure), clearDocsCookie()}, err: err}, nil
	}

	return clearedSession{clearRefreshCookie(h.cookieSecure), clearDocsCookie()}, nil
}

func toCurrentUser(u domain.User, permissions access.Set) openapi.CurrentUser {
	providers := make([]openapi.CurrentUserLinkedProviders, 0, len(u.LinkedProviders))
	for _, p := range u.LinkedProviders {
		providers = append(providers, openapi.CurrentUserLinkedProviders(p))
	}
	keys := make([]openapi.PermissionKey, 0, permissions.Len())
	for _, k := range permissions.Keys() {
		keys = append(keys, openapi.PermissionKey(k))
	}
	workspaces := make([]openapi.Workspace, 0, 3)
	for _, w := range access.Workspaces(permissions) {
		workspaces = append(workspaces, openapi.Workspace(w))
	}
	prefs := toAPIPreferences(u.Preferences)
	return openapi.CurrentUser{
		Id:                 httpapi.ParseUUID(u.ID),
		Email:              openapi_types.Email(u.Email),
		FullName:           u.FullName,
		DisplayName:        u.DisplayName,
		Phone:              u.Phone,
		Locale:             (*openapi.CurrentUserLocale)(u.Locale),
		TimeZone:           u.TimeZone,
		Preferences:        &prefs,
		Role:               openapi.Role(u.Role),
		HasPassword:        u.HasPassword(),
		LinkedProviders:    providers,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt,
		Permissions:        keys,
		Workspaces:         workspaces,
	}
}
