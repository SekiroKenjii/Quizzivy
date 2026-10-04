package http

import (
	"context"
	"errors"
	"net/http"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

const docsCookieName = "quizzivy_docs"

const legacyAdminRole = "admin"

// OpenDocsSession sets the caller's fifteen-minute docs session cookie, which
// RequireDocsSession accepts on /docs. The operation requires
// system.api_reference, which only the Admin holds, and the cookie carries the
// caller's session epoch.
func (h Identity) OpenDocsSession(ctx context.Context, _ openapi.OpenDocsSessionRequestObject) (openapi.OpenDocsSessionResponseObject, error) {
	if h.docs == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.New("docs session requested without an authenticated principal")
	}
	raw, err := h.docs.Issue(principal.UserID, legacyAdminRole, principal.Epoch)
	if err != nil {
		return nil, err
	}
	return openapi.OpenDocsSession204Response{
		Headers: openapi.OpenDocsSession204ResponseHeaders{SetCookie: httpapi.Ptr(docsCookie(raw).String())},
	}, nil
}

func clearDocsCookie() *http.Cookie {
	cleared := docsCookie("")
	cleared.MaxAge = -1
	return cleared
}

func docsCookie(raw string) *http.Cookie {
	return &http.Cookie{
		Name:     docsCookieName,
		Value:    raw,
		Path:     "/docs",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(token.DocsSessionTTL.Seconds()),
	}
}

// RequireDocsSession admits a request to the API reference only with the docs
// session cookie of a user who still holds system.api_reference: 401 without
// a valid cookie, for an unknown or disabled user, or for a cookie older than
// the user's session epoch; 403 when the user's role lacks the key. The
// Authorization header is never consulted.
func RequireDocsSession(docs *token.Issuer, resolver httpx.PrincipalResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(docsCookieName)
			if err != nil || cookie.Value == "" || docs == nil || resolver == nil {
				writeDocsUnauthenticated(w, r)
				return
			}
			claims, err := docs.Verify(cookie.Value)
			if err != nil {
				writeDocsUnauthenticated(w, r)
				return
			}
			resolved, err := resolver.Resolve(r.Context(), claims.Subject)
			switch {
			case errors.Is(err, httpx.ErrUnknownPrincipal):
				writeDocsUnauthenticated(w, r)
			case err != nil:
				httpx.WriteError(w, r, http.StatusInternalServerError, httpx.CodeInternal,
					httpx.TextFor(r, "Đã xảy ra lỗi. Vui lòng thử lại.", "Something went wrong. Try again."))
			case resolved.Disabled, claims.Epoch < resolved.Epoch:
				writeDocsUnauthenticated(w, r)
			case !resolved.Permissions.Has(access.SystemAPIReference):
				httpx.WriteError(w, r, http.StatusForbidden, httpx.CodeForbidden,
					httpx.TextFor(r, "Bạn không có quyền xem tài liệu API.", "You do not have permission to view the API reference."))
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

func writeDocsUnauthenticated(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized,
		httpx.TextFor(r, "Hãy mở tài liệu API từ trang Cài đặt của Quizzivy.", "Open the API reference from Quizzivy's Settings page."))
}
