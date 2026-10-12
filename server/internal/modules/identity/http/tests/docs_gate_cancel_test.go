package http_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identitytoken "quizzivy/internal/modules/identity/application/token"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

type failingPrincipals struct{ err error }

func (f failingPrincipals) Resolve(context.Context, string) (access.Principal, error) {
	return access.Principal{}, f.err
}

func docsGateAnswer(t *testing.T, request context.Context, resolveErr error) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	docs, err := identitytoken.NewDocsIssuer([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := docs.Issue("01935000-0000-7000-8000-0000000000d1", 0)
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	gate := identityhttp.RequireDocsSession(docs, failingPrincipals{resolveErr})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequestWithContext(request, http.MethodGet, "/docs", nil)
	req.AddCookie(&http.Cookie{Name: "quizzivy_docs", Value: raw})
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, req)
	return rec, reached
}

func TestTheDocsGateAnswersAResolveTheClientCancelledAs499(t *testing.T) {
	gone, cancel := context.WithCancel(context.Background())
	cancel()

	rec, reached := docsGateAnswer(t, gone, fmt.Errorf("read principal: %w", context.Canceled))

	if rec.Code != httpx.StatusClientClosedRequest {
		t.Errorf("status = %d, want 499", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none", rec.Body.String())
	}
	if reached {
		t.Error("the reference was served after the gate failed to resolve the caller")
	}
}

func TestTheDocsGateStillAnswers500ForAFaultThatIsNotTheClients(t *testing.T) {
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	cases := map[string]struct {
		request context.Context
		err     error
	}{
		"a server deadline":                       {context.Background(), fmt.Errorf("read principal: %w", context.DeadlineExceeded)},
		"a derived context cancelled":             {context.Background(), fmt.Errorf("read principal: %w", context.Canceled)},
		"a database fault on a cancelled request": {gone, errors.New("connection reset by peer")},
	}
	for name, c := range cases {
		rec, reached := docsGateAnswer(t, c.request, c.err)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s: status = %d, want 500", name, rec.Code)
		}
		if reached {
			t.Errorf("%s: the reference was served after the gate failed to resolve the caller", name)
		}
	}
}
