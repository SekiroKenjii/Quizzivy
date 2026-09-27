//go:build integration

package application_test

import (
	"encoding/base64"
	"errors"
	"quizzivy/internal/modules/identity/application/token"
	"strings"
	"testing"
	"time"
)

func issuer(t *testing.T) *token.Issuer {
	t.Helper()
	i, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func TestIssueAndVerify(t *testing.T) {
	i := issuer(t)
	tok, err := i.Issue("user-1", "admin", 0)
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

func TestTheSessionEpochRoundTripsAsSep(t *testing.T) {
	i := issuer(t)
	tok, err := i.Issue("user-1", "student", 3)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := i.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Epoch != 3 {
		t.Errorf("epoch = %d, want 3", claims.Epoch)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"sep":3`) {
		t.Errorf("payload %s carries no sep claim", payload)
	}
}

func TestATokenWithoutSepReadsAsEpochZero(t *testing.T) {
	i := issuer(t)
	tok, err := i.Issue("user-1", "student", 0)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "sep") {
		t.Errorf("an epoch-0 token carries sep: %s", payload)
	}
	claims, err := i.Verify(tok)
	if err != nil || claims.Epoch != 0 {
		t.Errorf("claims = %+v, err %v; want epoch 0", claims, err)
	}
}

func TestRejectsAShortSigningKey(t *testing.T) {
	if _, err := token.NewIssuer([]byte("too-short"), time.Minute); err == nil {
		t.Error("a 9-byte signing key was accepted")
	}
}

func TestExpiredTokenIsRejected(t *testing.T) {
	i := issuer(t)
	base := time.Now()
	i.SetClock(func() time.Time { return base })
	tok, _ := i.Issue("user-1", "student", 0)

	i.SetClock(func() time.Time { return base.Add(16 * time.Minute) })
	_, err := i.Verify(tok)
	if !errors.Is(err, token.ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestTokenFromADifferentKeyIsRejected(t *testing.T) {
	a := issuer(t)
	b, _ := token.NewIssuer([]byte(strings.Repeat("z", 32)), time.Minute)
	tok, _ := a.Issue("user-1", "admin", 0)
	if _, err := b.Verify(tok); err == nil {
		t.Error("a token signed with another key verified")
	}
}

func TestAlgNoneIsRejected(t *testing.T) {
	i := issuer(t)
	tok, _ := i.Issue("user-1", "student", 0)
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
	tok, _ := i.Issue("user-1", "student", 0)
	payload := strings.Split(tok, ".")[1]
	for _, forbidden := range []string{"email", "full_name", "fullName", "@"} {
		if strings.Contains(strings.ToLower(payload), strings.ToLower(forbidden)) {
			t.Errorf("token payload appears to contain %q", forbidden)
		}
	}
}
