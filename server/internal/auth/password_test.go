package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
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
	// The whole point of PHC format: a hash derived with parameters this
	// package would never choose must still verify. Tuning the constants above
	// must not invalidate every stored password.
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

	// And new hashes carry the current parameters, so a future reader can tell
	// which cost each stored password was made with.
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
//
// The two paths do identical work by construction -- BurnPasswordTime verifies
// against dummyHash, which init() builds with HashPassword, so the parameters
// are whatever the current ones are. This asserts that the construction has not
// been undone.
//
// SAMPLES ARE INTERLEAVED, one of each per round. Measuring seven of one and
// then seven of the other looks equivalent and is not: on a shared CI runner a
// slow patch -- CPU steal, a GC cycle, a noisy neighbour -- lands on whichever
// series happens to be running, and shifts an entire batch. That is what turned
// this red once, at 68ms against 157ms for two operations that differ only in
// which 64 MiB arena they touch. Interleaving makes both series experience the
// same conditions, so environmental noise cancels instead of accumulating on
// one side.
func TestUnknownUserCostsTheSameAsAWrongPassword(t *testing.T) {
	hash, err := HashPassword(context.Background(), "correct-horse")
	if err != nil {
		t.Fatal(err)
	}

	verify := func() { _, _ = VerifyPassword(context.Background(), "wrong", hash) }
	burn := func() { BurnPasswordTime(context.Background(), "wrong") }

	// One of each, untimed: the first Argon2id call faults in a fresh 64 MiB
	// arena, and that cost belongs to neither measurement.
	verify()
	burn()

	const rounds = 9
	verifySamples := make([]time.Duration, rounds)
	burnSamples := make([]time.Duration, rounds)
	for i := range rounds {
		verifySamples[i] = timeOne(verify)
		burnSamples[i] = timeOne(burn)
	}

	wrongPassword := medianOf(verifySamples)
	noSuchUser := medianOf(burnSamples)

	ratio := float64(noSuchUser) / float64(wrongPassword)
	if ratio < 0.5 || ratio > 2.0 {
		t.Errorf("timing differs too much: no-such-user %v vs wrong-password %v (ratio %.2f); "+
			"login would be a user-enumeration oracle\n  no-such-user samples:  %v\n  wrong-password samples: %v",
			noSuchUser, wrongPassword, ratio, burnSamples, verifySamples)
	}
}

func timeOne(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// medianOf sorts a copy, so the caller's samples stay in measurement order for
// the failure message -- which is where the interesting shape is.
func medianOf(samples []time.Duration) time.Duration {
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}
