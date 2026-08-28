package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTokenURL is Google's OAuth 2.0 token endpoint.
const DefaultTokenURL = "https://oauth2.googleapis.com/token"

var (
	// ErrExchangeFailed covers every way Google can refuse the code: expired,
	// already redeemed, a PKCE verifier that does not match, a mismatched
	// redirect. The client's response is identical in all of them -- start over
	// -- and the specific reason is Google's to know and ours to log.
	ErrExchangeFailed = errors.New("google: authorization code exchange failed")

	// ErrRedirectNotAllowed is a redirect_uri the deployment does not recognise.
	ErrRedirectNotAllowed = errors.New("google: redirect_uri is not allow-listed")
)

// Exchanger swaps an authorization code for an ID token. The client secret
// lives here and is never sent to a browser (§5.3).
type Exchanger struct {
	clientID     string
	clientSecret string
	tokenURL     string
	redirectURIs []string
	client       *http.Client
}

func NewExchanger(clientID, clientSecret string, redirectURIs []string, tokenURL string, client *http.Client) *Exchanger {
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Exchanger{
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenURL:     tokenURL,
		redirectURIs: redirectURIs,
		client:       client,
	}
}

// Exchange posts the code to Google and returns the raw ID token.
//
// redirectURI arrives from the browser and is therefore untrusted. Google does
// check it against the registered URIs for this client, so this second check is
// defence in depth -- but it is the cheap kind: it keeps a misconfigured or
// newly-registered redirect from becoming usable here without anyone deciding
// it should be.
func (e *Exchanger) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	if !e.redirectAllowed(redirectURI) {
		return "", fmt.Errorf("%w: %s", ErrRedirectNotAllowed, redirectURI)
	}

	form := url.Values{
		"code":          {code},
		"client_id":     {e.clientID},
		"client_secret": {e.clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrExchangeFailed, err)
	}
	defer resp.Body.Close()

	// Bounded: this is a response to an unauthenticated request, and an
	// endpoint that streams forever should not be able to exhaust our memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: reading response: %v", ErrExchangeFailed, err)
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &oauthErr)
		// Wrapped, so the detail reaches the log and not the caller.
		return "", fmt.Errorf("%w: status %d: %s: %s",
			ErrExchangeFailed, resp.StatusCode, oauthErr.Error, oauthErr.Description)
	}

	var token struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return "", fmt.Errorf("%w: decoding response: %v", ErrExchangeFailed, err)
	}
	if token.IDToken == "" {
		// A 200 with no id_token means the `openid` scope was not requested.
		// Worth its own message: it is a frontend configuration bug that would
		// otherwise present as an unexplained sign-in failure.
		return "", fmt.Errorf("%w: response carried no id_token (was the `openid` scope requested?)",
			ErrExchangeFailed)
	}
	return token.IDToken, nil
}

func (e *Exchanger) redirectAllowed(uri string) bool {
	for _, allowed := range e.redirectURIs {
		// Exact match. Prefix matching on redirect URIs is the classic OAuth
		// mistake: `https://app.example.com` would also allow
		// `https://app.example.com.attacker.test`.
		if uri == allowed {
			return true
		}
	}
	return false
}
