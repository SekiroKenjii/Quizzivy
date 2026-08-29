package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func issuer(t *testing.T) *TokenIssuer {
	t.Helper()
	i, err := NewTokenIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestIssueAndVerify(t *testing.T) {
	i := issuer(t)
	tok, err := i.Issue("user-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := i.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.Role != "admin" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestRejectsAShortSigningKey(t *testing.T) {
	if _, err := NewTokenIssuer([]byte("too-short"), time.Minute); err == nil {
		t.Error("a 9-byte signing key was accepted")
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	i := issuer(t)
	base := time.Now()
	i.SetClock(func() time.Time { return base })
	tok, _ := i.Issue("user-1", "student")

	i.SetClock(func() time.Time { return base.Add(16 * time.Minute) })
	_, err := i.Verify(tok)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestTokenFromADifferentKeyIsRejected(t *testing.T) {
	a := issuer(t)
	b, _ := NewTokenIssuer([]byte(strings.Repeat("z", 32)), time.Minute)
	tok, _ := a.Issue("user-1", "admin")
	if _, err := b.Verify(tok); err == nil {
		t.Error("a token signed with another key verified")
	}
}

func TestAlgNoneIsRejected(t *testing.T) {
	i := issuer(t)
	tok, _ := i.Issue("user-1", "student")
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape")
	}
	// {"alg":"none","typ":"JWT"} base64url, unpadded
	forged := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + parts[1] + "."
	if _, err := i.Verify(forged); err == nil {
		t.Error("a token with alg=none verified")
	}
}

func TestClaimsCarryNothingBeyondIdentityAndRole(t *testing.T) {
	i := issuer(t)
	tok, _ := i.Issue("user-1", "student")
	payload := strings.Split(tok, ".")[1]
	for _, forbidden := range []string{"email", "full_name", "fullName", "@"} {
		if strings.Contains(strings.ToLower(payload), strings.ToLower(forbidden)) {
			t.Errorf("token payload appears to contain %q", forbidden)
		}
	}
}
