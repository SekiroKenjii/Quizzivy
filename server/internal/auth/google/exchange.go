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
		return "", fmt.Errorf("%w: response carried no id_token (was the `openid` scope requested?)",
			ErrExchangeFailed)
	}
	return token.IDToken, nil
}

func (e *Exchanger) redirectAllowed(uri string) bool {
	for _, allowed := range e.redirectURIs {
		if uri == allowed {
			return true
		}
	}
	return false
}
