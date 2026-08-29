package api

import (
	"context"
	"errors"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/auth"
	"quizzivy/internal/httpx"
)

// Login implements POST /auth/login (§5.1).
//
// Every failure returns the same 401 with the same code and the same message.
// §6.5 requires the join endpoints not to reveal which classes exist; the same
// reasoning applies here to which accounts exist and which are suspended. The
// service layer additionally equalises the TIMING, which a response body alone
// cannot do.
func (s *Server) Login(ctx context.Context, request openapi.LoginRequestObject) (openapi.LoginResponseObject, error) {
	if s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return openapi.Login401JSONResponse(invalidCredentials(ctx)), nil
	}

	meta := httpx.RequestMetaFromContext(ctx)
	session, err := s.Deps.Auth.Login(ctx, auth.LoginInput{
		Email:     string(request.Body.Email),
		Password:  request.Body.Password,
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
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
			SetCookie: ptr(refreshCookie(session.RefreshToken, s.Deps.RefreshTTL, s.Deps.CookieSecure).String()),
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
			RequestId: parseUUID(httpx.RequestIDFromContext(ctx)),
		},
	}
}

// RefreshSession implements POST /auth/refresh (§5.2).
//
// The rotated cookie is the whole point of the response: the predecessor is
// revoked server-side before this returns, so a client that does not receive
// the replacement is already logged out and does not know it yet.
func (s *Server) RefreshSession(ctx context.Context, _ openapi.RefreshSessionRequestObject) (openapi.RefreshSessionResponseObject, error) {
	if s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	res, err := s.Deps.Auth.Refresh(ctx, auth.RefreshInput{
		Token:     refreshTokenFromContext(ctx),
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
	})
	switch {
	case errors.Is(err, auth.ErrRefreshReused):
		return openapi.RefreshSession401JSONResponse(authError(ctx, openapi.REFRESHTOKENREUSED,
			"Phiên đăng nhập này đã được sử dụng ở nơi khác. Vì lý do an toàn, vui lòng đăng nhập lại.")), nil
	case errors.Is(err, auth.ErrRefreshRejected):
		return openapi.RefreshSession401JSONResponse(authError(ctx, openapi.REFRESHTOKENINVALID,
			"Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.")), nil
	case err != nil:
		return nil, err
	}

	var body openapi.RefreshSession200JSONResponse
	body.Body.AccessToken = res.AccessToken
	body.Body.ExpiresIn = res.ExpiresIn
	body.Headers.SetCookie = ptr(refreshCookie(res.RefreshToken, s.Deps.RefreshTTL, s.Deps.CookieSecure).String())
	return body, nil
}

// Logout implements POST /auth/logout (§5.4).
//
// Authenticated by the refresh cookie rather than the access token: a user
// whose access token has already expired must still be able to end their
// session, and that is precisely when they are most likely to try.
func (s *Server) Logout(ctx context.Context, _ openapi.LogoutRequestObject) (openapi.LogoutResponseObject, error) {
	if s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}

	token := refreshTokenFromContext(ctx)
	if token == "" {
		return openapi.Logout401JSONResponse(authError(ctx, openapi.REFRESHTOKENINVALID,
			"Không có phiên đăng nhập.")), nil
	}
	if err := s.Deps.Auth.Logout(ctx, token); err != nil {
		return nil, err
	}

	return openapi.Logout204Response{
		Headers: openapi.Logout204ResponseHeaders{
			SetCookie: ptr(clearRefreshCookie(s.Deps.CookieSecure).String()),
		},
	}, nil
}

func authError(ctx context.Context, code openapi.ErrorCode, message string) openapi.ErrorResponse {
	return openapi.ErrorResponse{
		Error: struct {
			Code      openapi.ErrorCode       `json:"code"`
			Details   *map[string]interface{} `json:"details,omitempty"`
			Message   string                  `json:"message"`
			RequestId openapi.Uuid            `json:"requestId"`
		}{
			Code:      code,
			Message:   message,
			RequestId: parseUUID(httpx.RequestIDFromContext(ctx)),
		},
	}
}

func toAPIUser(u auth.User) openapi.User {
	providers := make([]openapi.UserLinkedProviders, 0, len(u.LinkedProviders))
	for _, p := range u.LinkedProviders {
		providers = append(providers, openapi.UserLinkedProviders(p))
	}
	return openapi.User{
		Id:                 parseUUID(u.ID),
		Email:              openapi_types.Email(u.Email),
		FullName:           u.FullName,
		Role:               openapi.Role(u.Role),
		HasPassword:        u.HasPassword(),
		LinkedProviders:    providers,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt,
	}
}

func ptr[T any](v T) *T { return &v }

// parseUUID converts a string id to the generated uuid type. Ids come from the
// database and from crypto/rand, so a parse failure is a programming error
// rather than user input; the zero value keeps the response renderable instead
// of turning a login failure into a 500.
func parseUUID(s string) openapi.Uuid {
	id, err := uuid.Parse(s)
	if err != nil {
		return openapi.Uuid{}
	}
	return openapi.Uuid(id)
}
