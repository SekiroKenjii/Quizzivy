package application_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"quizzivy/internal/modules/identity/application/token"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func docsIssuers(t *testing.T) (access, docs *token.Issuer) {
	t.Helper()
	key := []byte(strings.Repeat("k", 32))
	access, err := token.NewIssuer(key, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	docs, err = token.NewDocsIssuer(key)
	if err != nil {
		t.Fatal(err)
	}
	return access, docs
}

func TestADocsTokenOpensOnlyTheDocs(t *testing.T) {
	access, docs := docsIssuers(t)
	raw, err := docs.Issue("admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := docs.Verify(raw)
	if err != nil || claims.Subject != "admin-1" || claims.Role != "admin" {
		t.Fatalf("docs verify: %+v %v", claims, err)
	}
	if _, err := access.Verify(raw); !errors.Is(err, token.ErrTokenInvalid) {
		t.Fatalf("a docs token was accepted as an access token: %v", err)
	}
}

func TestAnAccessTokenNeverOpensTheDocs(t *testing.T) {
	access, docs := docsIssuers(t)
	raw, err := access.Issue("admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := docs.Verify(raw); !errors.Is(err, token.ErrTokenInvalid) {
		t.Fatalf("an access token opened the docs: %v", err)
	}
}

func TestADocsTokenLastsFifteenMinutes(t *testing.T) {
	_, docs := docsIssuers(t)
	start := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	docs.SetClock(func() time.Time { return start })
	raw, err := docs.Issue("admin-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	docs.SetClock(func() time.Time { return start.Add(token.DocsSessionTTL - time.Second) })
	if _, err := docs.Verify(raw); err != nil {
		t.Fatalf("still valid before expiry: %v", err)
	}
	docs.SetClock(func() time.Time { return start.Add(token.DocsSessionTTL + time.Second) })
	if _, err := docs.Verify(raw); !errors.Is(err, token.ErrTokenExpired) {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestTheDocsIssuerRefusesAShortKey(t *testing.T) {
	if _, err := token.NewDocsIssuer([]byte("short")); err == nil {
		t.Fatal("a 5-byte key was accepted")
	}
}

func signed(t *testing.T, key []byte, audience ...string) string {
	t.Helper()
	now := time.Now()
	claims := token.Claims{Role: "admin", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "admin-1", Issuer: "quizzivy",
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	}}
	if len(audience) > 0 {
		claims.Audience = audience
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAnAccessTokenIssuerRefusesAnyTokenThatNamesAnAudience(t *testing.T) {
	access, _ := docsIssuers(t)
	key := []byte(strings.Repeat("k", 32))
	if _, err := access.Verify(signed(t, key)); err != nil {
		t.Fatalf("a plain access token was refused: %v", err)
	}
	if _, err := access.Verify(signed(t, key, token.DocsAudience)); !errors.Is(err, token.ErrTokenInvalid) {
		t.Fatalf("a docs-audience token signed with the access key was accepted: %v", err)
	}
}

func TestTheDocsIssuerRefusesATokenWithoutItsAudience(t *testing.T) {
	_, docs := docsIssuers(t)
	derive := hmac.New(sha256.New, []byte(strings.Repeat("k", 32)))
	derive.Write([]byte("quizzivy docs session v1"))
	key := derive.Sum(nil)
	if _, err := docs.Verify(signed(t, key, token.DocsAudience)); err != nil {
		t.Fatalf("a docs token was refused: %v", err)
	}
	if _, err := docs.Verify(signed(t, key)); !errors.Is(err, token.ErrTokenInvalid) {
		t.Fatalf("a token without the docs audience opened the docs: %v", err)
	}
}
