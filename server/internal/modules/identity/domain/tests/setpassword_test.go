package domain_test

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/domain"
)

func TestASetPasswordTokenIs32RandomBytesAndTheDatabaseKeepsOnlyItsSHA256(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		token, hash, err := domain.NewSetPasswordToken()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil || len(raw) != 32 {
			t.Fatalf("token %q decodes to %d bytes (%v), want 32 bytes of unpadded base64url", token, len(raw), err)
		}
		if sum := sha256.Sum256([]byte(token)); !slices.Equal(hash, sum[:]) {
			t.Fatal("the hash is not the SHA-256 of the token's text")
		}
		if !slices.Equal(hash, domain.HashSetPasswordToken(token)) {
			t.Fatal("HashSetPasswordToken disagrees with the hash NewSetPasswordToken returned")
		}
		if seen[token] {
			t.Fatalf("token %q was issued twice", token)
		}
		seen[token] = true
	}
}

func TestALinkLivesTwentyFourHours(t *testing.T) {
	if domain.SetPasswordLifetime != 24*time.Hour {
		t.Fatalf("lifetime = %v, want 24h: migration 00134's check says the same", domain.SetPasswordLifetime)
	}
}

func TestOnlyTheTwoPurposesAreValid(t *testing.T) {
	for purpose, want := range map[domain.SetPasswordPurpose]bool{
		domain.LinkInvite: true, domain.LinkReset: true, "": false, "Invite": false, "other": false,
	} {
		if got := purpose.Valid(); got != want {
			t.Errorf("%q.Valid() = %v, want %v", purpose, got, want)
		}
	}
}

func TestALinkIsUsableOnlyWhileUnusedUnrevokedUnexpiredAndForAnEnabledAccount(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	earlier := now.Add(-time.Second)
	fresh := domain.SetPasswordLink{ExpiresAt: now.Add(time.Microsecond)}

	for name, c := range map[string]struct {
		change func(*domain.SetPasswordLink)
		want   bool
	}{
		"fresh":                     {func(*domain.SetPasswordLink) {}, true},
		"used":                      {func(l *domain.SetPasswordLink) { l.UsedAt = &earlier }, false},
		"revoked":                   {func(l *domain.SetPasswordLink) { l.RevokedAt = &earlier }, false},
		"expiring this instant":     {func(l *domain.SetPasswordLink) { l.ExpiresAt = now }, false},
		"expired a microsecond ago": {func(l *domain.SetPasswordLink) { l.ExpiresAt = now.Add(-time.Microsecond) }, false},
		"for a disabled account":    {func(l *domain.SetPasswordLink) { l.AccountDisabled = true }, false},
	} {
		link := fresh
		c.change(&link)
		if got := link.Usable(now); got != c.want {
			t.Errorf("%s: Usable = %v, want %v", name, got, c.want)
		}
	}
}
