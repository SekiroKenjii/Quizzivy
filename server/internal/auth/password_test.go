package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
)

// The hash the dev seed writes for both accounts (password: quizzivy-dev).
//
// Pinned here on purpose. It was generated before this package existed, which
// makes it a genuine cross-check that PHC format does what it is meant to: a
// hash produced elsewhere, with parameters this code never chose, still
// verifies. If someone tunes the constants above and this breaks, the format is
// not doing its job.
const seedHash = `$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4`

func TestVerifiesTheSeedHash(t *testing.T) {
	ok, err := VerifyPassword(context.Background(), "quizzivy-dev", seedHash)
	if err != nil {
		t.Fatalf("seed hash did not decode: %v", err)
	}
	if !ok {
		t.Error("the dev seed's password no longer verifies; seed/01-dev.sql and this package have diverged")
	}
}

func TestRoundTrip(t *testing.T) {
	hash, err := HashPassword(context.Background(), "mật khẩu của tôi")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash is not PHC argon2id: %.30s", hash)
	}

	ok, err := VerifyPassword(context.Background(), "mật khẩu của tôi", hash)
	if err != nil || !ok {
		t.Errorf("correct password did not verify (ok=%v err=%v)", ok, err)
	}

	ok, err = VerifyPassword(context.Background(), "mật khẩu khác", hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("wrong password verified")
	}
}

func TestSaltIsPerHash(t *testing.T) {
	a, _ := HashPassword(context.Background(), "same")
	b, _ := HashPassword(context.Background(), "same")
	if a == b {
		t.Error("two hashes of the same password are identical; the salt is not random")
	}
}

func TestRejectsMalformedHashesRatherThanReturningFalse(t *testing.T) {
	for name, encoded := range map[string]string{
		"empty":            "",
		"not PHC":          "just-a-string",
		"bcrypt":           "$2a$12$abcdefghijklmnopqrstuv",
		"wrong variant":    "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"bad version":      "$argon2id$v=16$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		"missing params":   "$argon2id$v=19$$c2FsdA$aGFzaA",
		"bad base64":       "$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA",
		"truncated fields": "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA",
	} {
		t.Run(name, func(t *testing.T) {
			ok, err := VerifyPassword(context.Background(), "anything", encoded)
			if err == nil {
				t.Errorf("malformed hash was accepted as decodable (ok=%v)", ok)
			}
			if ok {
				t.Error("malformed hash verified")
			}
		})
	}
}

func TestParametersTravelWithTheHash(t *testing.T) {
	salt := []byte("sixteen-byte-slt")
	weakKey := argon2.IDKey([]byte("x"), salt, 1, 8*1024, 1, 32)
	weak := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, 8*1024, 1, 1,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(weakKey))

	ok, err := VerifyPassword(context.Background(), "x", weak)
	if err != nil {
		t.Fatalf("a hash with non-default parameters failed to decode: %v", err)
	}
	if !ok {
		t.Error("a hash with non-default parameters did not verify")
	}
	fresh, err := HashPassword(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fresh, "m=65536,t=3,p=2") {
		t.Errorf("current parameters missing from a fresh hash: %.60s", fresh)
	}
}

// §6.5's reasoning applied to login: the endpoint must not reveal which
// accounts exist. A missing user that returns instantly, while a real one
// spends ~50ms hashing, is a user-enumeration oracle measurable over a handful
// of requests.

// TestDummyHashUsesTheCurrentCostParameters covers parameter drift.
//
// The equal-cost property holds structurally: BurnPasswordTime IS
// VerifyPassword against dummyHash, and dummyHash is produced at init by
// HashPassword, so raising the cost raises both paths together. This asserts
// that construction has not been undone -- by a hardcoded literal, say, or a
// dummyHash built with parameters of its own.
func TestDummyHashUsesTheCurrentCostParameters(t *testing.T) {
	p, _, _, err := decodeHash(dummyHash)
	if err != nil {
		t.Fatalf("dummyHash does not parse as Argon2id: %v", err)
	}
	if p.memory != defaultMemory || p.time != defaultTime || p.threads != defaultThreads {
		t.Errorf("dummyHash is m=%d,t=%d,p=%d but the current parameters are m=%d,t=%d,p=%d; "+
			"an unknown email would cost less than a real one and login would be a "+
			"user-enumeration oracle",
			p.memory, p.time, p.threads, defaultMemory, defaultTime, defaultThreads)
	}
}

// TestBurnPasswordTimeDoesRealWork covers the regression the structural
// argument does not reach: someone reading BurnPasswordTime as pointless work
// and optimising it away. Asserting dummyHash's parameters says nothing about
// whether anything still uses it.
func TestBurnPasswordTimeDoesRealWork(t *testing.T) {
	// Untimed: the first Argon2id call faults in a fresh 64 MiB arena.
	BurnPasswordTime(context.Background(), "warm-up")

	start := time.Now()
	BurnPasswordTime(context.Background(), "wrong")
	elapsed := time.Since(start)

	const floor = 5 * time.Millisecond
	if elapsed < floor {
		t.Errorf("BurnPasswordTime returned in %v, under the %v floor: it is no longer doing "+
			"the Argon2id work that makes an unknown email cost the same as a real one, "+
			"so login is a user-enumeration oracle", elapsed, floor)
	}
}

// BenchmarkPasswordPathsAreEqualCost keeps the number visible without gating CI
// on it. `go test -bench PasswordPaths ./internal/auth/` prints both; they
// should be within noise of each other, and a real divergence would show as a
// difference no amount of noise explains.
func BenchmarkPasswordPathsAreEqualCost(b *testing.B) {
	hash, err := HashPassword(context.Background(), "correct-horse")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("wrong-password", func(b *testing.B) {
		for b.Loop() {
			_, _ = VerifyPassword(context.Background(), "wrong", hash)
		}
	})
	b.Run("no-such-user", func(b *testing.B) {
		for b.Loop() {
			BurnPasswordTime(context.Background(), "wrong")
		}
	})
}
