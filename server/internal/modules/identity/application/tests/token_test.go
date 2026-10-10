//go:build integration

package application_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/identity/application/token"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
	tok, err := i.Issue("user-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := i.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestTheSessionEpochRoundTripsAsSep(t *testing.T) {
	i := issuer(t)
	tok, err := i.Issue("user-1", 3)
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
	tok, err := i.Issue("user-1", 0)
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
	tok, _ := i.Issue("user-1", 0)

	i.SetClock(func() time.Time { return base.Add(16 * time.Minute) })
	_, err := i.Verify(tok)
	if !errors.Is(err, token.ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestTokenFromADifferentKeyIsRejected(t *testing.T) {
	a := issuer(t)
	b, _ := token.NewIssuer([]byte(strings.Repeat("z", 32)), time.Minute)
	tok, _ := a.Issue("user-1", 0)
	if _, err := b.Verify(tok); err == nil {
		t.Error("a token signed with another key verified")
	}
}

func TestAlgNoneIsRejected(t *testing.T) {
	i := issuer(t)
	tok, _ := i.Issue("user-1", 0)
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

func TestClaimsCarryNothingBeyondIdentityAndEpoch(t *testing.T) {
	i := issuer(t)
	tok, _ := i.Issue("user-1", 3)
	raw, err := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[1])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	var got []string
	for name := range payload {
		got = append(got, name)
	}
	slices.Sort(got)
	want := []string{"exp", "iat", "iss", "nbf", "sep", "sub"}
	if !slices.Equal(got, want) {
		t.Errorf("token claims = %v, want %v", got, want)
	}
}

func TestATokenMintedBeforeTheRoleClaimWentStillVerifies(t *testing.T) {
	i := issuer(t)
	now := time.Now()
	minted := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "user-1",
		"role": "admin",
		"sep":  2,
		"iss":  "quizzivy",
		"iat":  now.Unix(),
		"nbf":  now.Unix(),
		"exp":  now.Add(time.Minute).Unix(),
	})
	raw, err := minted.SignedString([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := i.Verify(raw)
	if err != nil {
		t.Fatalf("a token that still names a role was refused: %v", err)
	}
	if claims.Subject != "user-1" || claims.Epoch != 2 {
		t.Errorf("claims = %+v, want subject user-1 and epoch 2", claims)
	}
}
