package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrTokenInvalid = errors.New("access token is invalid")

var ErrTokenExpired = errors.New("access token has expired")

// Claims is deliberately small. Anything put here is readable by anyone
// holding the token and cannot be revoked before it expires, so it carries
// identity and role only -- never email, name, or anything that changes.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Issuer struct {
	key      []byte
	ttl      time.Duration
	issuer   string
	audience string
	now      func() time.Time
}

// DocsAudience names the only thing a docs session token opens: the API reference.
const DocsAudience = "docs"

// DocsSessionTTL is how long a docs session lasts before the admin opens the reference again.
const DocsSessionTTL = 15 * time.Minute

func NewIssuer(signingKey []byte, ttl time.Duration) (*Issuer, error) {
	if len(signingKey) < 32 {
		return nil, fmt.Errorf("JWT signing key must be at least 32 bytes, got %d", len(signingKey))
	}
	if ttl <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}
	return &Issuer{key: signingKey, ttl: ttl, issuer: "quizzivy", now: time.Now}, nil
}

// NewDocsIssuer issues and verifies docs session tokens. They are signed with a
// key derived from the access-token key and carry the docs audience, so a docs
// token is never accepted as an access token and an access token never opens the docs.
func NewDocsIssuer(signingKey []byte) (*Issuer, error) {
	if len(signingKey) < 32 {
		return nil, fmt.Errorf("JWT signing key must be at least 32 bytes, got %d", len(signingKey))
	}
	derive := hmac.New(sha256.New, signingKey)
	derive.Write([]byte("quizzivy docs session v1"))
	return &Issuer{key: derive.Sum(nil), ttl: DocsSessionTTL, issuer: "quizzivy", audience: DocsAudience, now: time.Now}, nil
}

// SetClock replaces the time source. Tests only.
func (i *Issuer) SetClock(now func() time.Time) { i.now = now }

func (i *Issuer) TTL() time.Duration { return i.ttl }

func (i *Issuer) Issue(userID, role string) (string, error) {
	now := i.now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	if i.audience != "" {
		claims.Audience = jwt.ClaimStrings{i.audience}
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.key)
}

// Verify parses and validates a token. An issuer with an audience accepts only
// tokens for it; one without accepts only tokens that name no audience.
func (i *Issuer) Verify(raw string) (*Claims, error) {
	claims := &Claims{}
	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithTimeFunc(i.now),
	}
	if i.audience != "" {
		options = append(options, jwt.WithAudience(i.audience))
	}
	_, err := jwt.ParseWithClaims(
		raw, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
			}
			return i.key, nil
		},
		options...,
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: no subject", ErrTokenInvalid)
	}
	if i.audience == "" && len(claims.Audience) > 0 {
		return nil, fmt.Errorf("%w: token is for %v", ErrTokenInvalid, claims.Audience)
	}
	return claims, nil
}
