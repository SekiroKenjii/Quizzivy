package api

import (
	"context"
	"errors"

	"quizzivy/gen/openapi"
	"quizzivy/internal/auth"
	"quizzivy/internal/auth/google"
	"quizzivy/internal/httpx"
)

// GoogleAuth implements POST /auth/google (§5.3).
//
// **Leak review.** This endpoint is public and takes a bearer secret, so what it
// says about each failure matters. Everything before a successful token
// verification is one answer, 401: a bad code, a replayed code, a mismatched
// PKCE verifier, a redirect_uri we do not recognise, a forged or expired ID
// token, and an unverified email all look identical, because at that point the
// caller has proved nothing and telling them which check failed only helps them
// pass it next time.
//
// AFTER verification the answers become specific, and deliberately so: the
// caller has proved to Google that they control the address, so
// "your account is suspended" or "you need a class code" discloses nothing they
// could not confirm themselves -- and an opaque failure would leave a real
// student with no idea what to do next.
func (s *Server) GoogleAuth(ctx context.Context, request openapi.GoogleAuthRequestObject) (openapi.GoogleAuthResponseObject, error) {
	if s.Deps.Auth == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	in := auth.GoogleSignInInput{
		Code:         request.Body.Code,
		CodeVerifier: request.Body.CodeVerifier,
		RedirectURI:  request.Body.RedirectUri,
		UserAgent:    meta.UserAgent,
		IP:           meta.IP,
	}
	if request.Body.JoinCode != nil {
		in.JoinCode = *request.Body.JoinCode
	}

	var rejected auth.JoinCodeRejected
	result, err := s.Deps.Auth.GoogleSignIn(ctx, in)
	switch {
	case err == nil:

	// Nothing proved yet -- one answer for all of it.
	case errors.Is(err, google.ErrExchangeFailed),
		errors.Is(err, google.ErrRedirectNotAllowed),
		errors.Is(err, google.ErrTokenInvalid):
		return openapi.GoogleAuth401JSONResponse(authError(ctx, openapi.INVALIDCREDENTIALS,
			"Đăng nhập bằng Google không thành công. Vui lòng thử lại.")), nil

	// §5.1, non-negotiable. Its own message because it is the one failure the
	// user can actually fix, and only at Google.
	case errors.Is(err, google.ErrEmailUnverified):
		return openapi.GoogleAuth401JSONResponse(authError(ctx, openapi.EMAILNOTVERIFIED,
			"Địa chỉ email Google của bạn chưa được xác minh. Vui lòng xác minh với Google rồi thử lại.")), nil

	case errors.Is(err, auth.ErrAccountNotProvisioned):
		return openapi.GoogleAuth403JSONResponse(authError(ctx, openapi.ACCOUNTNOTPROVISIONED,
			"Tài khoản này chưa được đăng ký. Bạn cần mã lớp từ giáo viên để tham gia.")), nil

	case errors.Is(err, auth.ErrAccountDisabled):
		return openapi.GoogleAuth403JSONResponse(authError(ctx, openapi.ACCOUNTDISABLED,
			"Tài khoản của bạn đã bị vô hiệu hoá. Vui lòng liên hệ giáo viên.")), nil

	// The join code did not pass. No account was created: Enrol validates the
	// code before creating anything, in one transaction.
	case errors.As(err, &rejected):
		return openapi.GoogleAuth404JSONResponse(joinCodeError(ctx, rejected.Outcome)), nil

	case errors.Is(err, auth.ErrIdentityAlreadyLinked):
		return openapi.GoogleAuth403JSONResponse(authError(ctx, openapi.IDENTITYALREADYLINKED,
			"Tài khoản này đã được liên kết với một tài khoản Google khác.")), nil

	// Not configured, and not yet built. Both are our problem, not the
	// caller's, and neither should be dressed up as a failed sign-in.
	case errors.Is(err, auth.ErrGoogleUnavailable), errors.Is(err, auth.ErrSelfEnrolNotAvailable):
		return nil, httpx.ErrNotImplemented

	default:
		return nil, err
	}

	var response openapi.GoogleAuth200JSONResponse
	response.Body.AccessToken = result.Session.AccessToken
	response.Body.ExpiresIn = result.Session.ExpiresIn
	response.Body.User = toAPIUser(result.Session.User)
	// §5.3 step 5: the sign-in has no session without this.
	response.Headers.SetCookie = ptr(refreshCookie(
		result.Session.RefreshToken, s.Deps.RefreshTTL, s.Deps.CookieSecure).String())

	if c := result.EnrolledClass; c != nil {
		class := toAPIClass(*c)
		response.Body.EnrolledClass = &class
	}
	return response, nil
}

// LinkGoogle implements POST /auth/google/link (§15).
//
// Authenticated: this attaches a credential to an account that already exists,
// so the session is the proof of ownership and the Google exchange is the proof
// of the other side.
func (s *Server) LinkGoogle(ctx context.Context, request openapi.LinkGoogleRequestObject) (openapi.LinkGoogleResponseObject, error) {
	if s.Deps.Auth == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	user, err := s.Deps.Auth.LinkGoogle(ctx, auth.LinkGoogleInput{
		UserID:       principal.UserID,
		Code:         request.Body.Code,
		CodeVerifier: request.Body.CodeVerifier,
		RedirectURI:  request.Body.RedirectUri,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.LinkGoogle200JSONResponse(toAPIUser(user)), nil

	// Every way the link cannot be made. One code, because the client's move is
	// the same in all three: tell the user this Google account is not available
	// for this login.
	case errors.Is(err, auth.ErrIdentityAlreadyLinked),
		errors.Is(err, auth.ErrEmailBelongsToAnotherUser):
		return openapi.LinkGoogle409JSONResponse(authError(ctx, openapi.IDENTITYALREADYLINKED,
			"Tài khoản Google này không thể liên kết với tài khoản của bạn.")), nil

	case errors.Is(err, google.ErrEmailUnverified):
		return openapi.LinkGoogle401JSONResponse(authError(ctx, openapi.EMAILNOTVERIFIED,
			"Địa chỉ email Google của bạn chưa được xác minh. Vui lòng xác minh với Google rồi thử lại.")), nil

	case errors.Is(err, google.ErrExchangeFailed),
		errors.Is(err, google.ErrRedirectNotAllowed),
		errors.Is(err, google.ErrTokenInvalid):
		return openapi.LinkGoogle401JSONResponse(authError(ctx, openapi.INVALIDCREDENTIALS,
			"Liên kết Google không thành công. Vui lòng thử lại.")), nil

	case errors.Is(err, auth.ErrAccountDisabled), errors.Is(err, auth.ErrUserNotFound):
		return openapi.LinkGoogle401JSONResponse(sessionInvalid(ctx)), nil

	case errors.Is(err, auth.ErrGoogleUnavailable):
		return nil, httpx.ErrNotImplemented

	default:
		return nil, err
	}
}

// UnlinkGoogle implements DELETE /auth/google/link (§15).
//
// Refused when Google is the account's only way in. The result would be an
// account that still exists, still holds its attempts and enrolments, and that
// nobody can sign into -- including the person asking.
func (s *Server) UnlinkGoogle(ctx context.Context, _ openapi.UnlinkGoogleRequestObject) (openapi.UnlinkGoogleResponseObject, error) {
	if s.Deps.Auth == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	err := s.Deps.Auth.UnlinkGoogle(ctx, principal.UserID, meta.IP, meta.UserAgent)
	switch {
	case err == nil:
		return openapi.UnlinkGoogle204Response{}, nil
	case errors.Is(err, auth.ErrLastLoginMethod):
		return openapi.UnlinkGoogle409JSONResponse(authError(ctx, openapi.LASTLOGINMETHOD,
			"Bạn cần đặt mật khẩu trước khi bỏ liên kết Google, nếu không sẽ không còn cách nào đăng nhập.")), nil
	case errors.Is(err, auth.ErrAccountDisabled), errors.Is(err, auth.ErrUserNotFound):
		return openapi.UnlinkGoogle401JSONResponse{
			UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx)),
		}, nil
	default:
		return nil, err
	}
}
