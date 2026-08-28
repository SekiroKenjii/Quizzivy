package google_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"quizzivy/internal/auth/google"
)

const allowedRedirect = "https://app.quizzivy.com/auth/google/callback"

// fakeGoogle records what was posted and replies with whatever it is given.
func fakeGoogle(t *testing.T, status int, body any) (*httptest.Server, *url.Values) {
	t.Helper()
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestTheExchangeSendsWhatGoogleRequires(t *testing.T) {
	srv, posted := fakeGoogle(t, http.StatusOK, map[string]string{"id_token": "the.id.token"})
	e := google.NewExchanger(testClientID, "the-secret", []string{allowedRedirect}, srv.URL, srv.Client())

	token, err := e.Exchange(context.Background(), "the-code", "the-verifier", allowedRedirect)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if token != "the.id.token" {
		t.Errorf("id_token = %q", token)
	}

	for field, want := range map[string]string{
		"code":          "the-code",
		"client_id":     testClientID,
		"client_secret": "the-secret",
		"redirect_uri":  allowedRedirect,
		"grant_type":    "authorization_code",
		// PKCE. Without it, an intercepted code can be redeemed by whoever
		// intercepted it -- which is the entire reason §5.3 uses the code flow
		// rather than the implicit one.
		"code_verifier": "the-verifier",
	} {
		if got := posted.Get(field); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
}

func TestAnUnlistedRedirectIsRefusedBeforeAnyRequestIsMade(t *testing.T) {
	// redirect_uri comes from the browser. Google checks it too, but this check
	// is free and it fires before we hand a secret to anybody.
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(srv.Close)
	e := google.NewExchanger(testClientID, "the-secret", []string{allowedRedirect}, srv.URL, srv.Client())

	for _, uri := range []string{
		"https://evil.test/callback",
		// The classic prefix-matching bug: this is a different registrable
		// domain, and a `strings.HasPrefix` check would allow it.
		allowedRedirect + ".attacker.test",
		"https://app.quizzivy.com.attacker.test/auth/google/callback",
		"",
	} {
		if _, err := e.Exchange(context.Background(), "c", "v", uri); !errors.Is(err, google.ErrRedirectNotAllowed) {
			t.Errorf("redirect %q: error = %v, want ErrRedirectNotAllowed", uri, err)
		}
	}
	if called {
		t.Error("a rejected redirect still reached the token endpoint")
	}
}

func TestGooglesRefusalsAreOneAnswer(t *testing.T) {
	// invalid_grant covers an expired code, a replayed code, and a PKCE
	// verifier that does not match. The caller does the same thing in each
	// case -- start over -- and the detail belongs in our log, not in a
	// response to an unauthenticated caller.
	srv, _ := fakeGoogle(t, http.StatusBadRequest, map[string]string{
		"error":             "invalid_grant",
		"error_description": "Code was already redeemed.",
	})
	e := google.NewExchanger(testClientID, "s", []string{allowedRedirect}, srv.URL, srv.Client())

	_, err := e.Exchange(context.Background(), "used", "v", allowedRedirect)
	if !errors.Is(err, google.ErrExchangeFailed) {
		t.Fatalf("error = %v, want ErrExchangeFailed", err)
	}
}

func TestAResponseWithoutAnIdTokenIsAnError(t *testing.T) {
	// A 200 with no id_token means the frontend did not request the `openid`
	// scope. Treating it as success would produce an empty identity.
	srv, _ := fakeGoogle(t, http.StatusOK, map[string]string{"access_token": "only-this"})
	e := google.NewExchanger(testClientID, "s", []string{allowedRedirect}, srv.URL, srv.Client())

	if _, err := e.Exchange(context.Background(), "c", "v", allowedRedirect); !errors.Is(err, google.ErrExchangeFailed) {
		t.Fatalf("error = %v, want ErrExchangeFailed", err)
	}
}

func TestSeveralRedirectsCanBeAllowed(t *testing.T) {
	// One build serves localhost and production.
	const dev = "http://localhost:5173/auth/google/callback"
	srv, _ := fakeGoogle(t, http.StatusOK, map[string]string{"id_token": "t"})
	e := google.NewExchanger(testClientID, "s", []string{allowedRedirect, dev}, srv.URL, srv.Client())

	for _, uri := range []string{allowedRedirect, dev} {
		if _, err := e.Exchange(context.Background(), "c", "v", uri); err != nil {
			t.Errorf("redirect %q was refused: %v", uri, err)
		}
	}
}
