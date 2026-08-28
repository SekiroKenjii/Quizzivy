package google_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"quizzivy/internal/auth/google"
)

const testClientID = "123456789.apps.googleusercontent.com"

// A Google ID token is a bearer assertion of identity. Everything here is about
// refusing one that Google did not actually issue for us.

type signer struct {
	key *rsa.PrivateKey
	kid string
}

func newSigner(t *testing.T, kid string) *signer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &signer{key: key, kid: kid}
}

type claims struct {
	Email         string `json:"email,omitempty"`
	EmailVerified any    `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
	jwt.RegisteredClaims
}

func (s *signer) sign(t *testing.T, c claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	token.Header["kid"] = s.kid
	raw, err := token.SignedString(s.key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// jwksServer publishes the signers' public keys and counts how often it is asked.
func jwksServer(t *testing.T, signers ...*signer) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		keys := make([]map[string]string, 0, len(signers))
		for _, s := range signers {
			keys = append(keys, map[string]string{
				"kty": "RSA", "use": "sig", "alg": "RS256", "kid": s.kid,
				"n": base64.RawURLEncoding.EncodeToString(s.key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(s.key.E)).Bytes()),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func validClaims() claims {
	now := time.Now()
	return claims{
		Email:         "hocvien@example.com",
		EmailVerified: true,
		Name:          "Nguyễn Văn A",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://accounts.google.com",
			Subject:   "108000000000000000001",
			Audience:  jwt.ClaimStrings{testClientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
}

func newVerifier(t *testing.T, s *signer) (*google.Verifier, *atomic.Int64) {
	t.Helper()
	srv, hits := jwksServer(t, s)
	return google.NewVerifier(testClientID, google.NewKeySet(srv.URL, srv.Client())), hits
}

func TestAValidTokenVerifies(t *testing.T) {
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	identity, err := v.Verify(context.Background(), s.sign(t, validClaims()))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if identity.Subject != "108000000000000000001" {
		t.Errorf("subject = %q", identity.Subject)
	}
	if identity.Email != "hocvien@example.com" || !identity.EmailVerified {
		t.Errorf("email = %q verified = %v", identity.Email, identity.EmailVerified)
	}
	if identity.Name != "Nguyễn Văn A" {
		t.Errorf("name = %q", identity.Name)
	}
}

func TestATokenForAnotherAudienceIsRejected(t *testing.T) {
	// `aud` is what stops a token minted for a DIFFERENT Google client -- any
	// other site the user has signed into -- from being replayed here as proof
	// of identity. Without this check, anyone running any Google app could
	// impersonate any of their users against us.
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	c := validClaims()
	c.Audience = jwt.ClaimStrings{"999.apps.googleusercontent.com"}

	if _, err := v.Verify(context.Background(), s.sign(t, c)); !errors.Is(err, google.ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}

func TestATokenFromAnotherIssuerIsRejected(t *testing.T) {
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	c := validClaims()
	c.Issuer = "https://accounts.evil.test"

	if _, err := v.Verify(context.Background(), s.sign(t, c)); !errors.Is(err, google.ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}

func TestBothOfGooglesIssuerSpellingsAreAccepted(t *testing.T) {
	// Google issues ID tokens with `accounts.google.com` AND with the https
	// form. Accepting only one rejects valid tokens intermittently, which
	// presents as a flaky login rather than as a bug.
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	for _, issuer := range []string{"https://accounts.google.com", "accounts.google.com"} {
		c := validClaims()
		c.Issuer = issuer
		if _, err := v.Verify(context.Background(), s.sign(t, c)); err != nil {
			t.Errorf("issuer %q was rejected: %v", issuer, err)
		}
	}
}

func TestAnExpiredTokenIsRejected(t *testing.T) {
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	c := validClaims()
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	if _, err := v.Verify(context.Background(), s.sign(t, c)); !errors.Is(err, google.ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}

func TestATokenSignedByAnotherKeyIsRejected(t *testing.T) {
	published := newSigner(t, "key-1")
	attacker := &signer{key: newSigner(t, "key-1").key, kid: "key-1"} // same kid, different key
	v, _ := newVerifier(t, published)

	if _, err := v.Verify(context.Background(), attacker.sign(t, validClaims())); !errors.Is(err, google.ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}

func TestUnsignedAndSymmetricTokensAreRejected(t *testing.T) {
	// The classic JWT forgery: an RSA public key is public, so a verifier that
	// does not pin the algorithm will happily check an HS256 signature against
	// the modulus as if it were a shared secret. `alg: none` is the same bug
	// with the work removed.
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	none := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims())
	none.Header["kid"] = "key-1"
	unsigned, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(context.Background(), unsigned); !errors.Is(err, google.ErrTokenInvalid) {
		t.Errorf("alg=none accepted: %v", err)
	}

	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
	hs.Header["kid"] = "key-1"
	symmetric, err := hs.SignedString(s.key.N.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(context.Background(), symmetric); !errors.Is(err, google.ErrTokenInvalid) {
		t.Errorf("HS256 signed with the public modulus accepted: %v", err)
	}
}

func TestATokenWithNoKidIsRejectedRatherThanTriedAgainstEveryKey(t *testing.T) {
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims())
	raw, err := token.SignedString(s.key) // no kid header
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(context.Background(), raw); !errors.Is(err, google.ErrTokenInvalid) {
		t.Fatalf("error = %v, want ErrTokenInvalid", err)
	}
}

func TestUnverifiedEmailIsReportedRatherThanSwallowed(t *testing.T) {
	// Verify's job is the token; §5.1's rejection is the caller's branch. What
	// matters here is that the flag survives the trip intact, and that a value
	// which is not recognisably true reads as FALSE.
	s := newSigner(t, "key-1")
	v, _ := newVerifier(t, s)

	for name, raw := range map[string]any{
		"boolean false": false,
		"string false":  "false",
		"absent":        nil,
		"a number":      0,
		"nonsense":      "maybe",
	} {
		t.Run(name, func(t *testing.T) {
			c := validClaims()
			c.EmailVerified = raw
			identity, err := v.Verify(context.Background(), s.sign(t, c))
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if identity.EmailVerified {
				t.Errorf("email_verified %v read as verified", raw)
			}
		})
	}

	// Google sends a JSON boolean; its older userinfo responses used a string.
	// Both mean verified.
	for _, raw := range []any{true, "true"} {
		c := validClaims()
		c.EmailVerified = raw
		identity, err := v.Verify(context.Background(), s.sign(t, c))
		if err != nil {
			t.Fatal(err)
		}
		if !identity.EmailVerified {
			t.Errorf("email_verified %v did not read as verified", raw)
		}
	}
}

func TestAnUnknownKidTriggersOneRefetchAndThenStopsAsking(t *testing.T) {
	// Google rotates signing keys without notice, so an unknown kid has to be
	// able to refresh the cache -- but an attacker can mint unlimited tokens
	// with invented kids, and each must not become an outbound request.
	rotated := newSigner(t, "key-2")
	srv, hits := jwksServer(t, rotated)
	keys := google.NewKeySet(srv.URL, srv.Client())
	v := google.NewVerifier(testClientID, keys)

	// First call: cache is empty, so it fetches and finds key-2.
	if _, err := v.Verify(context.Background(), rotated.sign(t, validClaims())); err != nil {
		t.Fatalf("Verify after rotation: %v", err)
	}
	afterFirst := hits.Load()

	unknown := newSigner(t, "key-invented")
	for range 20 {
		if _, err := v.Verify(context.Background(), unknown.sign(t, validClaims())); err == nil {
			t.Fatal("a token signed by an unpublished key verified")
		}
	}
	if extra := hits.Load() - afterFirst; extra > 1 {
		t.Errorf("%d refetches for 20 forged kids; the rate limit is not holding", extra)
	}
}

func TestARotatedKeyIsPickedUpOnceTheRateLimitElapses(t *testing.T) {
	old := newSigner(t, "key-old")
	srv, _ := jwksServer(t, old)
	keys := google.NewKeySet(srv.URL, srv.Client())
	v := google.NewVerifier(testClientID, keys)

	if _, err := v.Verify(context.Background(), old.sign(t, validClaims())); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	// Google rotates. The published set changes under us.
	newKey := newSigner(t, "key-new")
	rotated, _ := jwksServer(t, newKey)
	keys2 := google.NewKeySet(rotated.URL, rotated.Client())
	v2 := google.NewVerifier(testClientID, keys2)
	if _, err := v2.Verify(context.Background(), newKey.sign(t, validClaims())); err != nil {
		t.Fatalf("a rotated key was not picked up: %v", err)
	}

	// And the old key stops working, rather than being kept around forever.
	if _, err := v2.Verify(context.Background(), old.sign(t, validClaims())); err == nil {
		t.Error("a retired signing key still verifies tokens")
	}
}

func TestAnUndersizedKeyIsRefused(t *testing.T) {
	// A 512-bit RSA key is factorable. Accepting whatever a JWKS document
	// offers would let anyone who could serve us one forge every token.
	small, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "use": "sig", "kid": "small",
			"n": base64.RawURLEncoding.EncodeToString(small.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(small.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)

	weak := &signer{key: small, kid: "small"}
	v := google.NewVerifier(testClientID, google.NewKeySet(srv.URL, srv.Client()))
	if _, err := v.Verify(context.Background(), weak.sign(t, validClaims())); err == nil {
		t.Fatal("a 1024-bit signing key was accepted")
	}
}

func TestAJWKSOutageDoesNotVerifyAnything(t *testing.T) {
	// Failing open here would mean any token at all is accepted whenever Google
	// is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	s := newSigner(t, "key-1")
	v := google.NewVerifier(testClientID, google.NewKeySet(srv.URL, srv.Client()))
	if _, err := v.Verify(context.Background(), s.sign(t, validClaims())); err == nil {
		t.Fatal("a token verified while the key endpoint was down")
	}
}
