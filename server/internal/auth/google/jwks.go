// Package google implements the §5.3 sign-in exchange: swap an authorization
// code for an ID token, and verify that token against Google's published keys.
package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// DefaultJWKSURL is Google's signing-key endpoint. Hard-coded rather than read
// from the discovery document: it has been stable for a decade, and one fewer
// network call on the sign-in path is one fewer thing to fail. Overridable so
// tests can point at a local server.
const DefaultJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

var ErrUnknownKey = errors.New("google: no signing key with that kid")

// KeySet caches Google's signing keys.
//
// Google rotates keys without warning, so a cache that only expires on a timer
// will serve an unknown `kid` for as long as its TTL. The cache therefore
// refreshes on demand when a token names a key it has not seen -- which is the
// only signal that a rotation happened -- and rate-limits that refresh so an
// attacker cannot turn a stream of forged `kid`s into a stream of outbound
// requests.
type KeySet struct {
	url    string
	client *http.Client
	now    func() time.Time

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	minInterval time.Duration
}

func NewKeySet(url string, client *http.Client) *KeySet {
	if url == "" {
		url = DefaultJWKSURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &KeySet{
		url:    url,
		client: client,
		now:    time.Now,
		keys:   map[string]*rsa.PublicKey{},
		// A forged `kid` must not become a request to Google. One refresh a
		// minute is far more than key rotation needs and far less than a flood.
		minInterval: time.Minute,
	}
}

// SetClock replaces the time source. Tests only.
func (k *KeySet) SetClock(now func() time.Time) { k.now = now }

// Key returns the public key for a kid, fetching once if it is unknown.
func (k *KeySet) Key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		// An ID token with no `kid` cannot be matched to a key. Choosing one
		// arbitrarily is how "try every key until one verifies" gets written,
		// which turns key rotation into a signature-stripping opportunity.
		return nil, fmt.Errorf("%w: token has no kid", ErrUnknownKey)
	}

	k.mu.RLock()
	key, ok := k.keys[kid]
	stale := k.now().Sub(k.fetchedAt) >= k.minInterval
	k.mu.RUnlock()
	if ok {
		return key, nil
	}
	if !stale {
		// Unknown kid, and we refreshed recently. Refusing without a fetch is
		// what keeps forged kids from being an outbound-request amplifier.
		return nil, fmt.Errorf("%w: %s", ErrUnknownKey, kid)
	}

	if err := k.refresh(ctx); err != nil {
		return nil, err
	}

	k.mu.RLock()
	defer k.mu.RUnlock()
	if key, ok := k.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownKey, kid)
}

func (k *KeySet) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return fmt.Errorf("google jwks request: %w", err)
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return fmt.Errorf("google jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google jwks fetch: status %d", resp.StatusCode)
	}

	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("google jwks decode: %w", err)
	}

	parsed := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		// Skip anything that is not an RSA signing key rather than failing the
		// whole refresh: Google publishing a key type we do not use must not
		// take sign-in down.
		if key.Kty != "RSA" || (key.Use != "" && key.Use != "sig") {
			continue
		}
		pub, err := key.rsaPublicKey()
		if err != nil {
			continue
		}
		parsed[key.Kid] = pub
	}
	if len(parsed) == 0 {
		// Replacing a working key set with an empty one would break every
		// subsequent sign-in until the next refresh.
		return errors.New("google jwks: response contained no usable RSA keys")
	}

	k.mu.Lock()
	k.keys = parsed
	k.fetchedAt = k.now()
	k.mu.Unlock()
	return nil
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (j jwk) rsaPublicKey() (*rsa.PublicKey, error) {
	modulus, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("jwk modulus: %w", err)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("jwk exponent: %w", err)
	}
	if len(exponent) == 0 || len(exponent) > 8 {
		return nil, errors.New("jwk exponent out of range")
	}
	// Google publishes 2048-bit keys. Anything materially smaller is either a
	// malformed document or an attempt to get a forgeable key accepted.
	if len(modulus) < 256 {
		return nil, fmt.Errorf("jwk modulus is %d bytes; refusing keys under 2048 bits", len(modulus)*8)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: int(new(big.Int).SetBytes(exponent).Int64()),
	}, nil
}
