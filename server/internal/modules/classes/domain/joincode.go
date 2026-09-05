package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"
)

// Generate returns a new code in canonical form (ungrouped, upper case).
//
// len(Alphabet) is 32, which divides 256, so masking a random byte with 31
// selects uniformly with no modulo bias and no rejection loop. If the alphabet
// ever changes length this stops being true, hence the assertion.
func (JoinCodeManager) Generate() (string, error) {
	if len(Alphabet) != 32 {
		return "", fmt.Errorf("join: alphabet is %d characters; uniform selection assumes 32", len(Alphabet))
	}

	raw := make([]byte, Length)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("join: generate code: %w", err)
	}

	out := make([]byte, Length)
	for i, b := range raw {
		out[i] = Alphabet[b&31]
	}
	return string(out), nil
}

// Format groups a code for display: XXXX-XXXX (§6.1).
func (JoinCodeManager) Format(code string) string {
	if len(code) != Length {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// Normalize turns what a person typed into the canonical form used for hashing.
func (JoinCodeManager) Normalize(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range strings.ToUpper(strings.TrimSpace(input)) {
		if strings.ContainsRune(Alphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hash is the stored form. SHA-256, not Argon2id: a code is 8 characters from a
// 32-symbol alphabet, so 40 bits of entropy from a CSPRNG. That is far too much
// to guess online at §6.5's rate limits, and unlike a password it is not chosen
// by a human, not reused elsewhere, and short-lived.
func (JoinCodeManager) Hash(normalized string) []byte {
	sum := sha256.Sum256([]byte(normalized))
	return sum[:]
}

// Equal compares two code hashes in constant time (§13.5).
//
// The lookup itself is a b-tree probe on code_hash and is not constant-time;
// this guards the place §13.5 actually names, and keeps the property if a
// caller ever compares two hashes it already holds.
func (JoinCodeManager) Equal(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// Hint is the last four characters, which is all that remains visible after the
// one-time reveal.
func (JoinCodeManager) Hint(normalized string) string {
	if len(normalized) < HintLength {
		return normalized
	}
	return normalized[len(normalized)-HintLength:]
}

// JoinCodeManager mints, normalises and hashes join codes; the plaintext is a value that exists once.
type JoinCodeManager struct{}

var JoinCodes JoinCodeManager

// Alphabet is §6.1's, exactly. Thirty-two characters, and the omissions are the
// point: `I` and `O` are gone because they read as `1` and `0`, and `1` and `0`
// are gone for the same reason.
const Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Length is the number of characters in a code, before grouping.
const Length = 8

// HintLength is how much of a code survives the one-time reveal, for the admin
// to recognise which code is active (§13.3).
const HintLength = 4

// CodeState is what decides whether a join code can still be redeemed.
type CodeState struct {
	SelfJoinEnabled bool
	RevokedAt       *time.Time
	ExpiresAt       time.Time
	MaxUses         *int
	UsesCount       int
}

// CodeRow is everything a preview decision needs, in one round trip.
type CodeRow struct {
	CodeState
	ClassID     string
	ClassName   string
	CodeHash    []byte
	TeacherName *string
}

// IssuedCode is the metadata of an active code. It never carries the plaintext:
// that exists only in the response to the request that created it (§13.3).
type IssuedCode struct {
	ID        string
	ClassID   string
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

// PreviewOutcome is why a code was or was not accepted. Each maps to a distinct
// error code so the student gets an accurate message -- "ask your teacher for a
// new code" is useful where "wrong code" is not -- without any of them naming a
// class (§6.5).
type PreviewOutcome int

const (
	PreviewOK PreviewOutcome = iota
	PreviewInvalid
	PreviewRevoked
	PreviewExpired
	PreviewExhausted
)

// PreviewResult carries exactly the three fields §6.5 permits, and the outcome.
type PreviewResult struct {
	Outcome     PreviewOutcome
	ClassID     string
	ClassName   string
	TeacherName string
}

// Usable reports PreviewOK or the reason the code is refused. A class with
// self-join closed answers exactly as a nonexistent code does, so the endpoints
// cannot be used to discover which classes exist.
func (c CodeState) Usable(now time.Time) PreviewOutcome {
	switch {
	case !c.SelfJoinEnabled:
		return PreviewInvalid
	case c.RevokedAt != nil:
		return PreviewRevoked
	case !c.ExpiresAt.After(now):
		return PreviewExpired
	case c.MaxUses != nil && c.UsesCount >= *c.MaxUses:
		return PreviewExhausted
	}
	return PreviewOK
}
