package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	sdk "github.com/anthropics/anthropic-sdk-go"
)

// The kinds of failure Complete reports. Every error it returns satisfies
// errors.Is against exactly one of them, except a cancelled caller, which gets
// context.Canceled unchanged.
var (
	// ErrRefused is a completion the provider ended with a refusal stop.
	ErrRefused = errors.New("anthropic: the model refused the request")
	// ErrRateLimited is a 429 that survived every retry.
	ErrRateLimited = errors.New("anthropic: rate limited")
	// ErrUnavailable is a 5xx or an overloaded stream that survived every retry,
	// or a connection that failed before the answer was complete.
	ErrUnavailable = errors.New("anthropic: the service is unavailable")
	// ErrTimeout is a call that outlived its time budget.
	ErrTimeout = errors.New("anthropic: the request timed out")
	// ErrInvalidResponse is an answer that is not one complete JSON object.
	ErrInvalidResponse = errors.New("anthropic: the answer is not a complete JSON object")
	// ErrAuth is a 401, 402 or 403: the key or the account, which only an
	// operator can fix.
	ErrAuth = errors.New("anthropic: the provider rejected the credentials or the account")
	// ErrInvalidRequest is any other 4xx, or a request the client refused to
	// send. It is never retried.
	ErrInvalidRequest = errors.New("anthropic: the request was rejected")
)

const (
	redacted         = "[redacted]"
	maxMessageLength = 300
)

// Error is the one concrete error type Complete returns. Kind is one of the
// sentinels above and Unwrap exposes it. Status is the HTTP status (0 when no
// response arrived) and RequestID the provider's request id when it sent one.
// Message is the provider's own explanation, with the API key removed and cut
// to a short line; nothing of the request or its headers is kept.
type Error struct {
	Kind      error
	Status    int
	RequestID string
	Message   string
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Kind.Error())
	if e.Status != 0 {
		b.WriteString(": status ")
		b.WriteString(http.StatusText(e.Status))
		b.WriteString(" (")
		b.WriteString(strconv.Itoa(e.Status))
		b.WriteString(")")
	}
	if e.RequestID != "" {
		b.WriteString(" request ")
		b.WriteString(e.RequestID)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Kind }

var errMalformedStream = errors.New("malformed event stream")

type failure struct {
	err        error
	retryAfter string
	retryable  bool
}

type scrubber struct{ key string }

func (s scrubber) clean(text string) string {
	if s.key != "" {
		text = strings.ReplaceAll(text, s.key, redacted)
	}
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > maxMessageLength {
		text = string(runes[:maxMessageLength]) + "..."
	}
	return text
}

func (s scrubber) newError(kind error, status int, requestID, message string) *Error {
	return &Error{Kind: kind, Status: status, RequestID: s.clean(requestID), Message: s.clean(message)}
}

func (s scrubber) fromSDK(err error, requestID string) failure {
	var api *sdk.Error
	switch {
	case errors.As(err, &api):
		return s.fromAPI(api, requestID)
	case errors.Is(err, errMalformedStream):
		return failure{err: s.newError(ErrInvalidResponse, 0, requestID, err.Error())}
	case errors.Is(err, context.DeadlineExceeded):
		return failure{err: s.newError(ErrTimeout, 0, requestID, "")}
	default:
		return failure{err: s.newError(ErrUnavailable, 0, requestID, err.Error())}
	}
}

func (s scrubber) fromAPI(api *sdk.Error, requestID string) failure {
	if api.RequestID != "" {
		requestID = api.RequestID
	}
	kind, retryable := classify(api.StatusCode, string(api.Type()))
	f := failure{err: s.newError(kind, api.StatusCode, requestID, providerMessage(api.RawJSON())), retryable: retryable}
	if api.Response != nil {
		f.retryAfter = api.Response.Header.Get("Retry-After")
	}
	return f
}

func classify(status int, errorType string) (kind error, retryable bool) {
	if status >= http.StatusBadRequest {
		switch {
		case status == http.StatusTooManyRequests:
			return ErrRateLimited, true
		case status >= http.StatusInternalServerError:
			return ErrUnavailable, true
		case status == http.StatusUnauthorized, status == http.StatusPaymentRequired, status == http.StatusForbidden:
			return ErrAuth, false
		}
		return ErrInvalidRequest, false
	}
	switch errorType {
	case "rate_limit_error":
		return ErrRateLimited, true
	case "authentication_error", "permission_error", "billing_error":
		return ErrAuth, false
	case "invalid_request_error", "not_found_error", "request_too_large":
		return ErrInvalidRequest, false
	}
	return ErrUnavailable, true
}

func providerMessage(raw string) string {
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(raw), &body) != nil {
		return ""
	}
	return body.Error.Message
}
