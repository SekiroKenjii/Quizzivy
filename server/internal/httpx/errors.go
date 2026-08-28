package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrNotImplemented marks an operation the contract declares but no phase has
// built yet. Rendered as 501 so an unbuilt endpoint is honestly unbuilt rather
// than a 404 that looks like a routing bug.
var ErrNotImplemented = errors.New("not implemented")

// ErrorCode mirrors the enum in api/openapi.yaml. Clients branch on this and
// nothing else; the message is already localised.
type ErrorCode string

const (
	CodeValidationFailed ErrorCode = "VALIDATION_FAILED"
	// 401 and 403 are different answers and the client acts differently on
	// each: UNAUTHORIZED means log in, FORBIDDEN means you are logged in and
	// still may not. This used to be one constant spelling both "FORBIDDEN",
	// which left the SPA guessing from the status line.
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeForbidden    ErrorCode = "FORBIDDEN"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeRateLimited  ErrorCode = "RATE_LIMITED"
	CodeInternal     ErrorCode = "INTERNAL"
)

// Error is the envelope from docs/plan/00-overview.md §7.
type Error struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"requestId"`
}

// WriteError renders the envelope. The requestId comes from the request
// context, so the id a user reads off the error screen is the same one in the
// server logs for that request.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Error{Error: ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: RequestIDFromContext(r.Context()),
	}})
}
