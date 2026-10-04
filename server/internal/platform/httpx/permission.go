package httpx

import (
	"context"
	"errors"
	"net/http"

	"quizzivy/internal/shared/access"
)

// ErrUnknownPrincipal is what a PrincipalResolver wraps when no user has the
// id a token names.
var ErrUnknownPrincipal = errors.New("httpx: unknown principal")

// PrincipalResolver answers who a user acts as: the role's permissions, the
// session epoch a token must carry and whether the account is disabled. It
// wraps ErrUnknownPrincipal when no user has the id.
type PrincipalResolver interface {
	Resolve(ctx context.Context, userID string) (access.Principal, error)
}

// RequirePermission enforces each operation's declared permission, keyed by
// route pattern as PermissionRequirements builds them. A route with no
// requirement is open and passes untouched. Any other request needs the
// principal RequireAuth put in the context: without one, or when the user is
// unknown, disabled or the token's epoch is older than the user's, it answers
// 401 so the single-flight refresh settles the session; when the user's
// permissions do not meet the requirement it answers 403 FORBIDDEN. On success
// the resolved principal joins the context.
func RequirePermission(requirements map[string]access.Requirement, resolver PrincipalResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requirement, gated := requirements[r.Pattern]
			if !gated {
				next.ServeHTTP(w, r)
				return
			}
			caller, ok := PrincipalFromContext(r.Context())
			if !ok {
				writeUnauthenticated(w, r)
				return
			}
			resolved, err := resolver.Resolve(r.Context(), caller.UserID)
			switch {
			case errors.Is(err, ErrUnknownPrincipal):
				writeUnauthenticated(w, r)
				return
			case err != nil:
				WriteError(w, r, http.StatusInternalServerError, CodeInternal,
					TextFor(r, "Đã xảy ra lỗi. Vui lòng thử lại.", "Something went wrong. Try again."))
				return
			case resolved.Disabled, caller.Epoch < resolved.Epoch:
				writeUnauthenticated(w, r)
				return
			case !requirement.SatisfiedBy(resolved.Permissions):
				writeForbidden(w, r)
				return
			}
			caller.Access = resolved
			next.ServeHTTP(w, withPrincipal(r, caller))
		})
	}
}

func writeForbidden(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusForbidden, CodeForbidden,
		TextFor(r, "Bạn không có quyền truy cập chức năng này.", "You do not have permission to use this feature."))
}
