package ratelimit

import (
	"context"
	"net/http"
)

type principalKey struct{}

// WithPrincipal records on the request the id of the user the permission gate
// resolved, for PrincipalKey to read back.
func WithPrincipal(r *http.Request, userID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, userID))
}

// PrincipalKey is the KeyFunc for the user id WithPrincipal recorded, or ""
// when none was, so a bucket keyed by it counts per signed-in user and is
// skipped where no user was resolved.
func PrincipalKey(r *http.Request) string {
	userID, _ := r.Context().Value(principalKey{}).(string)
	return userID
}
