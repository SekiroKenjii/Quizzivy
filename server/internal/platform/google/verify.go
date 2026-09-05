package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid    = errors.New("google: id token invalid")
	ErrEmailUnverified = errors.New("google: email is not verified")
)

var googleIssuers = []string{"https://accounts.google.com", "accounts.google.com"}

// Identity is what §5.3 step 3 reads out of a verified ID token.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type Verifier struct {
	clientID string
	keys     *KeySet
	now      func() time.Time
}

func NewVerifier(clientID string, keys *KeySet) *Verifier {
	return &Verifier{clientID: clientID, keys: keys, now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (v *Verifier) SetClock(now func() time.Time) { v.now = now }

type idTokenClaims struct {
	Email         string       `json:"email"`
	EmailVerified flexibleBool `json:"email_verified"`
	Name          string       `json:"name"`
	Picture       string       `json:"picture"`
	jwt.RegisteredClaims
}

// Verify checks signature, issuer, audience and expiry, then returns the
// identity. It does NOT decide whether the email is verified -- that is the
// caller's branch, because §5.3's resolution order needs to reject it before
// looking anything up.
func (v *Verifier) Verify(ctx context.Context, rawIDToken string) (Identity, error) {
	claims := &idTokenClaims{}

	_, err := jwt.ParseWithClaims(
		rawIDToken, claims, func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			return v.keys.Key(ctx, kid)
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithAudience(v.clientID),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(v.now),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	if !validIssuer(claims.Issuer) {
		return Identity{}, fmt.Errorf("%w: issuer %q", ErrTokenInvalid, claims.Issuer)
	}
	if claims.Subject == "" {
		return Identity{}, fmt.Errorf("%w: no subject", ErrTokenInvalid)
	}
	if claims.Email == "" {
		return Identity{}, fmt.Errorf("%w: no email", ErrTokenInvalid)
	}

	return Identity{
		Subject:       claims.Subject,
		Email:         strings.TrimSpace(claims.Email),
		EmailVerified: bool(claims.EmailVerified),
		Name:          strings.TrimSpace(claims.Name),
		Picture:       claims.Picture,
	}, nil
}

func validIssuer(iss string) bool {
	for _, valid := range googleIssuers {
		if iss == valid {
			return true
		}
	}
	return false
}

type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	var asBool bool
	if err := json.Unmarshal(data, &asBool); err == nil {
		*b = flexibleBool(asBool)
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*b = flexibleBool(asString == "true")
		return nil
	}
	*b = false
	return nil
}
