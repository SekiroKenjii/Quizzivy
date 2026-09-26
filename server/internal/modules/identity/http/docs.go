package http

import (
	"context"
	"errors"
	"net/http"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

const docsCookieName = "quizzivy_docs"

// OpenDocsSession sets the caller's fifteen-minute docs session cookie, which
// RequireDocsSession accepts on /docs; the /admin/ prefix makes it admin only.
func (h Identity) OpenDocsSession(ctx context.Context, _ openapi.OpenDocsSessionRequestObject) (openapi.OpenDocsSessionResponseObject, error) {
	if h.docs == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, errors.New("docs session requested without an authenticated principal")
	}
	raw, err := h.docs.Issue(principal.UserID, principal.Role)
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

// RequireDocsSession admits a request to the API reference only with an admin's
// docs session cookie: 401 without a valid one, 403 for any other role. The
// Authorization header is never consulted.
func RequireDocsSession(docs *token.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(docsCookieName)
			if err != nil || cookie.Value == "" || docs == nil {
				writeDocsUnauthenticated(w, r)
				return
			}
			claims, err := docs.Verify(cookie.Value)
			if err != nil {
				writeDocsUnauthenticated(w, r)
				return
			}
			if claims.Role != httpx.RoleAdmin {
				httpx.WriteError(w, r, http.StatusForbidden, httpx.CodeForbidden,
					"Bạn không có quyền xem tài liệu API.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeDocsUnauthenticated(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized,
		"Hãy mở tài liệu API từ trang Cài đặt của Quizzivy.")
}
