package httpx_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func pastDeadline() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	cancel()
	return ctx
}

type clientGoneCase struct {
	name string
	ctx  context.Context
	err  error
	want bool
}

func writeTimeout() error {
	return fmt.Errorf("write failed: %w", &net.OpError{Op: "write", Net: "tcp", Err: os.ErrDeadlineExceeded})
}

func clientGoneCases() []clientGoneCase {
	return []clientGoneCase{
		{"the client cancelled and the call says so", cancelled(), fmt.Errorf("query: %w", context.Canceled), true},
		{"the same, wrapped twice", cancelled(), fmt.Errorf("tests: %w", fmt.Errorf("query: %w", context.Canceled)), true},
		{"the same, joined with another failure", cancelled(), errors.Join(errors.New("rollback"), context.Canceled), true},
		{"the driver's write was interrupted by the client's cancel and reports the timeout", cancelled(), fmt.Errorf("query: %w", writeTimeout()), true},
		{"the same, as a bare deadline error", cancelled(), fmt.Errorf("query: %w", os.ErrDeadlineExceeded), true},
		{"a network timeout and the client is still there", context.Background(), fmt.Errorf("query: %w", writeTimeout()), false},
		{"a network timeout on a request whose own deadline passed", pastDeadline(), fmt.Errorf("query: %w", writeTimeout()), false},
		{"a network timeout that names a server deadline, on a request the client also cancelled", cancelled(), fmt.Errorf("query: %w", errors.Join(writeTimeout(), context.DeadlineExceeded)), false},
		{"a context the server derived was cancelled and the client is still there", context.Background(), fmt.Errorf("query: %w", context.Canceled), false},
		{"a deadline the server imposed passed and the client is still there", context.Background(), fmt.Errorf("query: %w", context.DeadlineExceeded), false},
		{"a server deadline passed on a request the client also cancelled", cancelled(), fmt.Errorf("query: %w", context.DeadlineExceeded), false},
		{"the request's own deadline passed", pastDeadline(), fmt.Errorf("query: %w", context.DeadlineExceeded), false},
		{"the request's own deadline passed and the call reports a cancel", pastDeadline(), fmt.Errorf("query: %w", context.Canceled), false},
		{"another failure on a cancelled request", cancelled(), errors.New("connection reset by peer"), false},
		{"no failure on a cancelled request", cancelled(), nil, false},
		{"no failure", context.Background(), nil, false},
	}
}

func TestClientGoneIsACancelledRequestWhoseFailureSaysSo(t *testing.T) {
	for _, c := range clientGoneCases() {
		if got := httpx.ClientGone(c.ctx, c.err); got != c.want {
			t.Errorf("%s: ClientGone = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWriteClientGoneAnswersTheNginxStatusWithNoBody(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteClientGone(rec)
	if rec.Code != 499 || httpx.StatusClientClosedRequest != 499 {
		t.Errorf("status = %d, constant = %d, want both 499", rec.Code, httpx.StatusClientClosedRequest)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none: the envelope has no code for a failure that is not the server's", rec.Body.String())
	}
}

type failingResolver struct{ err error }

func (f failingResolver) Resolve(context.Context, string) (access.Principal, error) {
	return access.Principal{}, f.err
}

func gatedCall(ctx context.Context, resolver httpx.PrincipalResolver) (*httptest.ResponseRecorder, bool) {
	reached := false
	h := gateBefore(resolver, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/teacher", nil)
	req.Pattern = "GET /teacher"
	req.Header.Set("Authorization", "Bearer grader@0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, reached
}

func TestThePermissionGateAnswersAResolveTheClientCancelledAs499(t *testing.T) {
	rec, reached := gatedCall(cancelled(), failingResolver{fmt.Errorf("read principal: %w", context.Canceled)})
	if rec.Code != httpx.StatusClientClosedRequest {
		t.Errorf("status = %d, want 499: a client's own cancellation is not a server error", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none", rec.Body.String())
	}
	if reached {
		t.Error("the handler ran after the gate failed to resolve the caller")
	}
}

func TestThePermissionGateStillAnswers500ForAFaultThatIsNotTheClients(t *testing.T) {
	for _, c := range clientGoneCases() {
		if c.err == nil || c.want {
			continue
		}
		rec, reached := gatedCall(c.ctx, failingResolver{c.err})
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s: status = %d, want 500", c.name, rec.Code)
		}
		if reached {
			t.Errorf("%s: the handler ran after the gate failed to resolve the caller", c.name)
		}
	}
}

func TestThePermissionGateStillTreatsAnUnknownUserAsUnauthenticatedWhenTheClientIsGone(t *testing.T) {
	rec, _ := gatedCall(cancelled(), failingResolver{fmt.Errorf("lookup: %w", httpx.ErrUnknownPrincipal)})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401: a user the database does not know is not the client's cancellation", rec.Code)
	}
}
