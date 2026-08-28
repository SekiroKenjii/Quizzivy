package api

import (
	"context"
	"errors"
	"net/http"
	"time"

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
		// Anything else is an operational fault -- a corrupt stored hash, the
		// database being unreachable. Reporting it as bad credentials would
		// send the user to reset a password that was never wrong.
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

// refreshCookie builds the §5.2 cookie: httpOnly, Secure, SameSite=Lax,
// Path=/auth, and HOST-ONLY -- no Domain attribute, so only the API host ever
// receives it.
//
// SameSite=Lax is sufficient because app.quizzivy.com and api.quizzivy.com are
// the same SITE even though they are different origins. On genuinely cross-site
// hosts this cookie would never be sent and sessions would die silently
// (docs/plan/30-risks.md R-07).
func refreshCookie(token string, ttl time.Duration, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     "quizzivy_refresh",
		Value:    token,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
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
