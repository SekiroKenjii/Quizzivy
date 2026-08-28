package auth

import (
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
	ok, err := VerifyPassword("quizzivy-dev", seedHash)
	if err != nil {
		t.Fatalf("seed hash did not decode: %v", err)
	}
	if !ok {
		t.Error("the dev seed's password no longer verifies; seed/01-dev.sql and this package have diverged")
	}
}

func TestRoundTrip(t *testing.T) {
	hash, err := HashPassword("mật khẩu của tôi")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash is not PHC argon2id: %.30s", hash)
	}

	ok, err := VerifyPassword("mật khẩu của tôi", hash)
	if err != nil || !ok {
		t.Errorf("correct password did not verify (ok=%v err=%v)", ok, err)
	}

	ok, err = VerifyPassword("mật khẩu khác", hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("wrong password verified")
	}
}

func TestSaltIsPerHash(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("two hashes of the same password are identical; the salt is not random")
	}
}

func TestRejectsMalformedHashesRatherThanReturningFalse(t *testing.T) {
	// Returning (false, nil) for a corrupt hash would look like a wrong
	// password, and a storage problem would be diagnosed as a user error.
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
			ok, err := VerifyPassword("anything", encoded)
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
	// The whole point of PHC format: a hash derived with parameters this
	// package would never choose must still verify. Tuning the constants above
	// must not invalidate every stored password.
	salt := []byte("sixteen-byte-slt")
	weakKey := argon2.IDKey([]byte("x"), salt, 1, 8*1024, 1, 32)
	weak := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, 8*1024, 1, 1,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(weakKey))

	ok, err := VerifyPassword("x", weak)
	if err != nil {
		t.Fatalf("a hash with non-default parameters failed to decode: %v", err)
	}
	if !ok {
		t.Error("a hash with non-default parameters did not verify")
	}

	// And new hashes carry the current parameters, so a future reader can tell
	// which cost each stored password was made with.
	fresh, err := HashPassword("x")
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
func TestUnknownUserCostsTheSameAsAWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}

	median := func(fn func()) time.Duration {
		const runs = 7
		samples := make([]time.Duration, runs)
		for i := range samples {
			start := time.Now()
			fn()
			samples[i] = time.Since(start)
		}
		for i := 1; i < len(samples); i++ {
			for j := i; j > 0 && samples[j] < samples[j-1]; j-- {
				samples[j], samples[j-1] = samples[j-1], samples[j]
			}
		}
		return samples[runs/2]
	}

	wrongPassword := median(func() { _, _ = VerifyPassword("wrong", hash) })
	noSuchUser := median(func() { BurnPasswordTime("wrong") })

	ratio := float64(noSuchUser) / float64(wrongPassword)
	if ratio < 0.5 || ratio > 2.0 {
		t.Errorf("timing differs too much: no-such-user %v vs wrong-password %v (ratio %.2f); "+
			"login would be a user-enumeration oracle", noSuchUser, wrongPassword, ratio)
	}
}
