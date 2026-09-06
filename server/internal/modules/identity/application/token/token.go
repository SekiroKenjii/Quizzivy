package token

import (
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
	key    []byte
	ttl    time.Duration
	issuer string
	now    func() time.Time
}

func NewIssuer(signingKey []byte, ttl time.Duration) (*Issuer, error) {
	if len(signingKey) < 32 {
		return nil, fmt.Errorf("JWT signing key must be at least 32 bytes, got %d", len(signingKey))
	}
	if ttl <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}
	return &Issuer{key: signingKey, ttl: ttl, issuer: "quizzivy", now: time.Now}, nil
}

// SetClock replaces the time source. Tests only.
func (i *Issuer) SetClock(now func() time.Time) { i.now = now }

func (i *Issuer) TTL() time.Duration { return i.ttl }

func (i *Issuer) Issue(userID, role string) (string, error) {
	now := i.now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
			NotBefore: jwt.NewNumericDate(now),
		},
	})
	return token.SignedString(i.key)
}

// Verify parses and validates a token.
func (i *Issuer) Verify(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(
		raw, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
			}
			return i.key, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithTimeFunc(i.now),
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
	return claims, nil
}
