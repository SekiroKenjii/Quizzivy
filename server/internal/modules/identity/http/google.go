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
			"Đăng nhập bằng Google không thành công. Vui lòng thử lại.")), nil
	case errors.Is(err, domain.ErrGoogleEmailUnverified):
		return openapi.GoogleAuth401JSONResponse(httpapi.Error(ctx, openapi.EMAILNOTVERIFIED,
			"Địa chỉ email Google của bạn chưa được xác minh. Vui lòng xác minh với Google rồi thử lại.")), nil

	case errors.Is(err, domain.ErrAccountNotProvisioned):
		return openapi.GoogleAuth403JSONResponse(httpapi.Error(ctx, openapi.ACCOUNTNOTPROVISIONED,
			"Tài khoản này chưa được đăng ký. Bạn cần mã lớp từ giáo viên để tham gia.")), nil

	case errors.Is(err, domain.ErrAccountDisabled):
		return openapi.GoogleAuth403JSONResponse(httpapi.Error(ctx, openapi.ACCOUNTDISABLED,
			"Tài khoản của bạn đã bị vô hiệu hoá. Vui lòng liên hệ giáo viên.")), nil
	case errors.As(err, &rejected):
		return openapi.GoogleAuth404JSONResponse(classeshttp.JoinCodeError(ctx, rejected.Outcome)), nil

	case errors.Is(err, domain.ErrIdentityAlreadyLinked):
		return openapi.GoogleAuth403JSONResponse(httpapi.Error(ctx, openapi.IDENTITYALREADYLINKED,
			"Tài khoản này đã được liên kết với một tài khoản Google khác.")), nil
	case errors.Is(err, domain.ErrGoogleUnavailable), errors.Is(err, domain.ErrSelfEnrolNotAvailable):
		return nil, httpx.ErrNotImplemented

	default:
		return nil, err
	}

	var response openapi.GoogleAuth200JSONResponse
	response.Body.AccessToken = result.Session.AccessToken
	response.Body.ExpiresIn = result.Session.ExpiresIn
	response.Body.User = toAPIUser(result.Session.User)

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
		return openapi.LinkGoogle200JSONResponse(toAPIUser(user)), nil
	case errors.Is(err, domain.ErrIdentityAlreadyLinked),
		errors.Is(err, domain.ErrEmailBelongsToAnotherUser):
		return openapi.LinkGoogle409JSONResponse(httpapi.Error(ctx, openapi.IDENTITYALREADYLINKED,
			"Tài khoản Google này không thể liên kết với tài khoản của bạn.")), nil

	case errors.Is(err, domain.ErrGoogleEmailUnverified):
		return openapi.LinkGoogle401JSONResponse(httpapi.Error(ctx, openapi.EMAILNOTVERIFIED,
			"Địa chỉ email Google của bạn chưa được xác minh. Vui lòng xác minh với Google rồi thử lại.")), nil

	case errors.Is(err, domain.ErrGoogleExchangeFailed),
		errors.Is(err, domain.ErrGoogleRedirectNotAllowed),
		errors.Is(err, domain.ErrGoogleTokenInvalid):
		return openapi.LinkGoogle401JSONResponse(httpapi.Error(ctx, openapi.INVALIDCREDENTIALS,
			"Liên kết Google không thành công. Vui lòng thử lại.")), nil

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
			"Bạn cần đặt mật khẩu trước khi bỏ liên kết Google, nếu không sẽ không còn cách nào đăng nhập.")), nil
	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.UnlinkGoogle401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	default:
		return nil, err
	}
}
