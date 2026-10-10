package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	classeshttp "quizzivy/internal/modules/classes/http"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/actor"
)

// GoogleAuth completes the §5.3 sign-in: verify the ID token, then resolve the
// account by provider identity, then by verified email, then create one.
func (h Identity) GoogleAuth(ctx context.Context, request openapi.GoogleAuthRequestObject) (openapi.GoogleAuthResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	in := command.GoogleSignIn{
		Code:         request.Body.Code,
		CodeVerifier: request.Body.CodeVerifier,
		RedirectURI:  request.Body.RedirectUri,
		UserAgent:    meta.UserAgent,
		IP:           meta.IP,
		GeoLabel:     GeoLabelFromContext(ctx),
	}
	if request.Body.JoinCode != nil {
		in.JoinCode = *request.Body.JoinCode
	}

	var rejected domain.JoinCodeRejected
	result, err := h.app.Commands.GoogleSignIn.Handle(ctx, in)
	switch {
	case err == nil:

	case errors.Is(err, domain.ErrGoogleExchangeFailed),
		errors.Is(err, domain.ErrGoogleRedirectNotAllowed),
		errors.Is(err, domain.ErrGoogleTokenInvalid):
		return openapi.GoogleAuth401JSONResponse(httpapi.Error(ctx, openapi.INVALIDCREDENTIALS,
			httpx.Text(ctx, "Đăng nhập bằng Google không thành công. Vui lòng thử lại.",
				"Google sign-in did not work. Please try again."))), nil
	case errors.Is(err, domain.ErrGoogleEmailUnverified):
		return openapi.GoogleAuth401JSONResponse(httpapi.Error(ctx, openapi.EMAILNOTVERIFIED,
			httpx.Text(ctx, "Địa chỉ email Google của bạn chưa được xác minh. Vui lòng xác minh với Google rồi thử lại.",
				"Your Google email address is not verified. Please verify it with Google and try again."))), nil

	case errors.Is(err, domain.ErrAccountNotProvisioned):
		return openapi.GoogleAuth403JSONResponse(httpapi.Error(ctx, openapi.ACCOUNTNOTPROVISIONED,
			httpx.Text(ctx, "Tài khoản này chưa được đăng ký. Bạn cần mã lớp từ giáo viên để tham gia.",
				"This account is not registered. You need a class code from your teacher to join."))), nil

	case errors.Is(err, domain.ErrAccountDisabled):
		return openapi.GoogleAuth403JSONResponse(httpapi.Error(ctx, openapi.ACCOUNTDISABLED,
			httpx.Text(ctx, "Tài khoản của bạn đã bị vô hiệu hoá. Vui lòng liên hệ giáo viên.",
				"Your account has been disabled. Please contact your teacher."))), nil
	case errors.As(err, &rejected):
		return openapi.GoogleAuth404JSONResponse(classeshttp.JoinCodeError(ctx, rejected.Outcome)), nil

	case errors.Is(err, domain.ErrIdentityAlreadyLinked):
		return openapi.GoogleAuth403JSONResponse(httpapi.Error(ctx, openapi.IDENTITYALREADYLINKED,
			httpx.Text(ctx, "Tài khoản này đã được liên kết với một tài khoản Google khác.",
				"This account is already linked to another Google account."))), nil
	case errors.Is(err, domain.ErrGoogleUnavailable), errors.Is(err, domain.ErrSelfEnrolNotAvailable):
		return nil, httpx.ErrNotImplemented

	default:
		return nil, err
	}

	var response openapi.GoogleAuth200JSONResponse
	response.Body.AccessToken = result.Session.AccessToken
	response.Body.ExpiresIn = result.Session.ExpiresIn
	response.Body.User = h.currentUser(ctx, result.Session.User, result.Session.Permissions)

	response.Headers.SetCookie = httpapi.Ptr(refreshCookie(
		result.Session.RefreshToken, h.refreshTTL, h.cookieSecure).String())

	if c := result.EnrolledClass; c != nil {
		class := classeshttp.ToAPIClass(*c)
		response.Body.EnrolledClass = &class
	}
	return response, nil
}

// LinkGoogle implements POST /auth/google/link (§15).
func (h Identity) LinkGoogle(ctx context.Context, request openapi.LinkGoogleRequestObject) (openapi.LinkGoogleResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	user, err := h.app.Commands.LinkGoogle.Handle(ctx, command.LinkGoogle{UserID: principal.UserID,
		Code:         request.Body.Code,
		CodeVerifier: request.Body.CodeVerifier,
		RedirectURI:  request.Body.RedirectUri,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.LinkGoogle200JSONResponse(h.currentUser(ctx, user, principal.Access.Permissions)), nil
	case errors.Is(err, domain.ErrIdentityAlreadyLinked),
		errors.Is(err, domain.ErrEmailBelongsToAnotherUser):
		return openapi.LinkGoogle409JSONResponse(httpapi.Error(ctx, openapi.IDENTITYALREADYLINKED,
			httpx.Text(ctx, "Tài khoản Google này không thể liên kết với tài khoản của bạn.",
				"This Google account cannot be linked to your account."))), nil

	case errors.Is(err, domain.ErrGoogleEmailUnverified):
		return openapi.LinkGoogle401JSONResponse(httpapi.Error(ctx, openapi.EMAILNOTVERIFIED,
			httpx.Text(ctx, "Địa chỉ email Google của bạn chưa được xác minh. Vui lòng xác minh với Google rồi thử lại.",
				"Your Google email address is not verified. Please verify it with Google and try again."))), nil

	case errors.Is(err, domain.ErrGoogleExchangeFailed),
		errors.Is(err, domain.ErrGoogleRedirectNotAllowed),
		errors.Is(err, domain.ErrGoogleTokenInvalid):
		return openapi.LinkGoogle401JSONResponse(httpapi.Error(ctx, openapi.INVALIDCREDENTIALS,
			httpx.Text(ctx, "Liên kết Google không thành công. Vui lòng thử lại.",
				"Linking Google did not work. Please try again."))), nil

	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.LinkGoogle401JSONResponse(sessionInvalid(ctx)), nil

	case errors.Is(err, domain.ErrGoogleUnavailable):
		return nil, httpx.ErrNotImplemented

	default:
		return nil, err
	}
}

// UnlinkGoogle implements DELETE /auth/google/link (§15).
func (h Identity) UnlinkGoogle(ctx context.Context, _ openapi.UnlinkGoogleRequestObject) (openapi.UnlinkGoogleResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	_, err := h.app.Commands.UnlinkGoogle.Handle(ctx, command.UnlinkGoogle{Actor: actor.Actor{ID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent}})
	switch {
	case err == nil:
		return openapi.UnlinkGoogle204Response{}, nil
	case errors.Is(err, domain.ErrLastLoginMethod):
		return openapi.UnlinkGoogle409JSONResponse(httpapi.Error(ctx, openapi.LASTLOGINMETHOD,
			httpx.Text(ctx, "Bạn cần đặt mật khẩu trước khi bỏ liên kết Google, nếu không sẽ không còn cách nào đăng nhập.",
				"You need to set a password before unlinking Google, or there will be no way left to sign in."))), nil
	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.UnlinkGoogle401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	default:
		return nil, err
	}
}
